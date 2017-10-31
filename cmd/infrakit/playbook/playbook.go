package playbook

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/cmd/infrakit/base"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/cli/local"
	"github.com/docker/infrakit/pkg/cli/remote"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/playbook")

const (
	// PlaybooksFileEnvVar is the location of the playbooks file
	PlaybooksFileEnvVar = "INFRAKIT_PLAYBOOKS_FILE"
)

func init() {
	base.Register(Command)
}

type playbook struct {
	// Source is the original location
	Source string

	// Cache is the cached location, as a url of file:// format
	Cache string
}

type playbooks map[remote.Op]*playbook

func (pb *playbooks) module() map[remote.Op]remote.SourceURL {
	module := map[remote.Op]remote.SourceURL{}
	for k, p := range *pb {
		if p.Cache != "" {
			module[k] = remote.SourceURL(p.Cache)
		} else {
			module[k] = remote.SourceURL(p.Source)
		}
	}
	return module
}

func (pb *playbooks) writeTo(path string) error {
	any, err := types.AnyValue(*pb)
	if err != nil {
		return err
	}
	buff, err := any.MarshalYAML()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, buff, 0755)
}

func (pb *playbooks) loadFrom(path string) error {
	buff, err := ioutil.ReadFile(defaultPlaybooksFile())
	if err != nil {
		if !os.IsExist(err) {
			return nil
		}
		return err
	}
	yaml, err := types.AnyYAML(buff)
	if err != nil {
		return err
	}
	return yaml.Decode(pb)
}

func defaultPlaybooksFile() string {
	if playbooksFile := os.Getenv(PlaybooksFileEnvVar); playbooksFile != "" {
		return playbooksFile
	}

	// if there's INFRAKIT_HOME defined
	home := os.Getenv("INFRAKIT_HOME")
	if home != "" {
		return filepath.Join(home, "playbooks.yml")
	}

	home = os.Getenv("HOME")
	if usr, err := user.Current(); err == nil {
		home = usr.HomeDir
	}
	return filepath.Join(home, ".infrakit/playbooks.yml")
}

// Load loads the playbook
func Load() (remote.Modules, error) {
	return loadPlaybooks()
}

func loadPlaybooks() (remote.Modules, error) {
	pb := &playbooks{}
	err := pb.loadFrom(defaultPlaybooksFile())
	if err != nil {
		return nil, err
	}
	return pb.module(), nil
}

// Command is the entrypoint
func Command(scope scope.Scope) *cobra.Command {

	///////////////////////////////////////////////////////////////////////////////////
	// playbook
	cmd := &cobra.Command{
		Use:   "playbook",
		Short: "Manage playbooks",
	}
	quiet := cmd.PersistentFlags().BoolP("quiet", "q", false, "Print rows without column headers")

	cache := true

	add := &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a playbook",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 2 {
				cmd.Usage()
				os.Exit(1)
			}

			name := args[0]
			source := args[1]

			pb := playbooks{}
			err := pb.loadFrom(defaultPlaybooksFile())
			if err != nil {
				return err
			}

			if _, has := pb[remote.Op(name)]; has {
				return fmt.Errorf("%s already exists", name)
			}

			// try fetch
			_, err = template.Fetch(source, template.Options{})
			if err != nil {
				return err
			}

			cacheDir := ""

			// if caching then fetch the whole bundle
			if cache && !(strings.Contains(source, "file://") || strings.Contains(source, "str://")) {

				u, err := url.Parse(source)
				if err != nil {
					return err
				}

				// Build the commands here... while we turn on caching so that
				// templates are written to local cache

				cacheDir = filepath.Join(template.Dir(), name)
				test, err := remote.NewModules(scope,
					map[remote.Op]remote.SourceURL{
						remote.Op(name): remote.SourceURL(source),
					},
					os.Stdin, template.Options{
						CacheDir: cacheDir,
					})
				if err != nil {
					return err
				}
				cmds, err := test.List()
				if err != nil {
					return err
				}

				// update the cacheDir to be the url form
				cacheDir = "file://" + filepath.Join(cacheDir, u.Path)

				fmt.Println("found", len(cmds), "commands", "cached", cacheDir)
			}

			pb[remote.Op(name)] = &playbook{
				Source: source,
				Cache:  cacheDir,
			}

			return pb.writeTo(defaultPlaybooksFile())
		},
	}
	add.Flags().BoolVarP(&cache, "cache", "c", cache, "Cache the playbook")

	remove := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a playbook",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			name := args[0]

			pb := playbooks{}
			err := pb.loadFrom(defaultPlaybooksFile())
			if err != nil {
				return err
			}
			delete(pb, remote.Op(name))
			return pb.writeTo(defaultPlaybooksFile())
		},
	}

	update := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a cached playbook",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 1 {
				cmd.Usage()
				os.Exit(1)
			}

			name := args[0]

			pb := playbooks{}
			err := pb.loadFrom(defaultPlaybooksFile())
			if err != nil {
				return err
			}

			p, has := pb[remote.Op(name)]
			if has && p.Cache != "" {

				// remove then add
				err := remove.RunE(nil, []string{name})
				if err != nil {
					return err
				}
				fmt.Println("Cleared cache. Updating")
				return add.RunE(nil, []string{name, string(p.Source)})
			}
			return nil
		},
	}

	rawOutputFlags, rawOutput := cli.Output()
	list := &cobra.Command{
		Use:   "ls",
		Short: "List playbooks",
		RunE: func(cmd *cobra.Command, args []string) error {

			if len(args) != 0 {
				cmd.Usage()
				os.Exit(1)
			}

			pb := playbooks{}
			err := pb.loadFrom(defaultPlaybooksFile())
			if err != nil {
				return err
			}

			return rawOutput(os.Stdout, pb,
				func(io.Writer, interface{}) error {
					if !*quiet {
						fmt.Printf("%-20s\t%-50s\t%-50s\n", "PLAYBOOK", "URL", "CACHE")
					}

					for op, pb := range pb {
						fmt.Printf("%-20v\t%-50v\t%-50s\n", op, pb.Source, pb.Cache)
					}
					return nil
				})
		},
	}
	list.Flags().AddFlagSet(rawOutputFlags)

	cmd.AddCommand(add, remove, update, list)

	reserved := map[*cobra.Command]int{add: 1, remove: 1, list: 1}

	// Modules
	mods := []*cobra.Command{}
	// additional modules
	if os.Getenv(cli.CliDirEnvVar) != "" {
		modules, err := local.NewModules(scope, local.Dir())
		if err != nil {
			log.Crit("error executing", "err", err)
			os.Exit(1)
		}
		localModules, err := modules.List()
		log.Debug("modules", "local", localModules)
		if err != nil {
			log.Crit("error executing", "err", err)
			os.Exit(1)
		}
		mods = append(mods, localModules...)
	}

	// any remote modules?
	pmod, err := Load()
	if err != nil {
		log.Warn("playbooks failed to load", "err", err)
	} else {
		if playbooks, err := remote.NewModules(scope, pmod, os.Stdin, template.Options{}); err != nil {
			log.Warn("error loading playbooks", "err", err)
		} else {
			if more, err := playbooks.List(); err != nil {
				log.Warn("cannot list playbooks", "err", err)
			} else {
				mods = append(mods, more...)
			}
		}
	}

	for _, mod := range mods {
		if _, has := reserved[mod]; has {
			log.Warn("cannot override reserverd command; igored", "conflict", mod.Use)
			continue
		}

		log.Debug("Adding", "module", mod.Use)
		cmd.AddCommand(mod)
	}

	return cmd
}
