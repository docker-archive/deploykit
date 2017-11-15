package playbook

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/infrakit/cmd/infrakit/base"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/cli/playbook"
	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/docker/infrakit/pkg/template"
	"github.com/spf13/cobra"
)

var log = logutil.New("module", "cli/playbook")

func init() {
	base.Register(Command)
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

			playbooks, err := playbook.Load()
			if err != nil {
				return err
			}

			if playbooks.Has(playbook.Op(name)) {
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
				test, err := playbook.NewModules(scope,
					map[playbook.Op]playbook.SourceURL{
						playbook.Op(name): playbook.SourceURL(source),
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

			playbooks.Add(playbook.Op(name), source, cacheDir)

			return playbooks.Save()
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

			playbooks, err := playbook.Load()
			if err != nil {
				return err
			}
			playbooks.Remove(playbook.Op(name))
			return playbooks.Save()
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

			playbooks, err := playbook.Load()
			if err != nil {
				return err
			}

			op := playbook.Op(name)
			if playbooks.Has(op) {
				// remove then add
				err := remove.RunE(nil, []string{name})
				if err != nil {
					return err
				}
				fmt.Println("Cleared cache. Updating")
				return add.RunE(nil, []string{name, playbooks.Source(op)})
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

			playbooks, err := playbook.Load()
			if err != nil {
				return err
			}

			return rawOutput(os.Stdout, playbooks,
				func(io.Writer, interface{}) error {
					if !*quiet {
						fmt.Printf("%-20s\t%-50s\t%-50s\n", "PLAYBOOK", "URL", "CACHE")
					}

					playbooks.Visit(func(op playbook.Op, pb playbook.Playbook) {
						fmt.Printf("%-20v\t%-50v\t%-50s\n", op, pb.Source, pb.Cache)
					})
					return nil
				})
		},
	}
	list.Flags().AddFlagSet(rawOutputFlags)

	cmd.AddCommand(add, remove, update, list)

	// reserved := map[*cobra.Command]int{add: 1, remove: 1, list: 1}

	// // Commands from playbooks
	// playbookCommands := []*cobra.Command{}

	// // playbooks
	// pb, err := playbook.Load()
	// if err != nil {
	// 	log.Warn("playbooks failed to load", "err", err)
	// }

	// if playbooks, err := playbook.NewModules(scope, pb.Modules(), os.Stdin, template.Options{}); err != nil {
	// 	log.Warn("error loading playbooks", "err", err)
	// } else {
	// 	if more, err := playbooks.List(); err != nil {
	// 		log.Warn("cannot list playbooks", "err", err)
	// 	} else {
	// 		playbookCommands = append(playbookCommands, more...)
	// 	}
	// }

	// for _, c := range playbookCommands {
	// 	if _, has := reserved[c]; has {
	// 		log.Warn("cannot override reserverd command; igored", "conflict", c.Use)
	// 		continue
	// 	}

	// 	log.Warn("Adding", "module", c.Use)
	// 	cmd.AddCommand(c)
	// }

	return cmd
}
