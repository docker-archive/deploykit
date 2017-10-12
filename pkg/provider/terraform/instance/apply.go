package instance

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	manager_discovery "github.com/docker/infrakit/pkg/manager/discovery"
	"github.com/docker/infrakit/pkg/types"
	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/spf13/afero"
)

// terraformApply starts a goroutine that executes "terraform apply" at the
// configured freqency; if the goroutine is already running then the sleeping
// is interrupted
func (p *plugin) terraformApply() error {
	if p.pretend {
		return nil
	}

	p.applyLock.Lock()
	defer p.applyLock.Unlock()

	if p.applying {
		select {
		case p.pollChannel <- true:
			log.Infoln("Successfully interrupted terraform apply goroutine")
		default:
			log.Infoln("Polling channel is full, not interrupting")
		}
		return nil
	}

	p.pollChannel = make(chan bool, 1)
	go func() {
		initial := true
		for {
			// Conditionally apply terraform
			if p.shouldApply() {
				fns := tfFuncs{
					tfRefresh: func() error {
						command := exec.Command("terraform refresh").
							InheritEnvs(true).
							WithEnvs(p.envs...).
							WithDir(p.Dir)
						if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
							return err
						}
						return command.Wait()
					},
					tfStateList: p.doTerraformStateList,
				}
				// The trigger for an apply is typically from a group commit, sleep for a few seconds so
				// that multiple .tf.json.new files have time to be created
				if initial {
					time.Sleep(time.Second * 5)
				}
				if err := p.handleFiles(fns); err == nil {
					if err = p.doTerraformApply(); err == nil {
						initial = false
					} else {
						log.Errorf("Failed to execute 'terraform apply': %v", err)
					}
				} else {
					log.Errorf("Not executing 'terraform apply' due to error: %v", err)
				}
			} else {
				log.Infof("Not applying terraform, checking again in %v seconds", p.pollInterval)
			}

			select {
			case <-p.pollChannel:
				// Interrupted, use same initial delay so that more than a single delta
				// can be processed
				initial = true
				break
			case <-time.After(p.pollInterval):
				break
			}
		}
	}()

	p.applying = true
	return nil
}

// doTerraformApply executes "terraform apply"
func (p *plugin) doTerraformApply() error {
	log.Infoln("Applying plan")
	command := exec.Command("terraform apply -refresh=false").
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start()
	if err == nil {
		return command.Wait()
	}
	return err
}

// shouldApply returns true if "terraform apply" should execute; this happens if
// either the plugin is configured to be standalone or if the associated manager
// plugin is the current leader.
func (p *plugin) shouldApply() bool {
	// If there is no lookup func then the plugin is running standalone
	if p.pluginLookup == nil {
		return true
	}
	manager, err := manager_discovery.Locate(p.pluginLookup)
	if err != nil {
		log.Errorf("Failed to locate manager plugin: %v", err)
		return false
	}
	isLeader, err := manager.IsLeader()
	if err != nil {
		log.Errorf("Failed to determine manager leadership: %v", err)
		return false
	}
	if isLeader {
		log.Debugf("Running on leader manager, applying terraform")
		return true
	}
	log.Infof("Not running on leader manager, not applying terraform")
	return false
}

// External functions to use during when pruning files; broken out for testing
type tfFuncs struct {
	tfRefresh   func() error
	tfStateList func() (map[TResourceType]map[TResourceName]struct{}, error)
}

// handleFiles handles resource pruning and new resources via:
// 1. Acquire file system lock
// 2. Execute "terraform refresh" to refresh state
// 3. Identity and remove ".tf.json" files that are in the terraform state
// 4. Remove all "tf.json.new" files to "tf.json"
// Once these steps are done then "terraform apply" can execute without the
// file system lock.
func (p *plugin) handleFiles(fns tfFuncs) (err error) {
	if err := p.fsLock.TryLock(); err != nil {
		log.Infof("In handleFiles, cannot acquire file lock")
		return err
	}
	defer p.fsLock.Unlock()

	// Reading the files on NFS can cause panics, handles these here so that the
	// calling goroute does not die.
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("Panic detected: %v, %s", r, debug.Stack())
			err = fmt.Errorf("Panic detected: %v", r)
		}
	}()

	// Refresh resources and get updated resources names
	if err := fns.tfRefresh(); err != nil {
		return err
	}
	tfStateResources, err := fns.tfStateList()
	if err != nil {
		// TODO(kaufers): If it possible that not all of the .new files were moved to
		//  .tf.json files (NFS connection could be lost) and this could make the refresh
		//  always fail due to references that are not valid. Update this flow to still
		//  rename .new files even if the refresh fails (but do not prune or apply since
		//  we need valid refresh'd data) and then let the next iteration attempt to
		//  reconsile things.
		return err
	}

	// Get all instance files and all new files
	tfInstFiles := map[TResourceType]map[TResourceName]string{}
	tfNewFiles := map[string]struct{}{}
	fs := &afero.Afero{Fs: p.fs}
	err = fs.Walk(p.Dir,
		func(path string, info os.FileInfo, err error) error {
			// Only the VM files are valid for pruning; once pruned then the group controller polling will
			// ensure that a replacement is created. There is no mechanism that ensures consistency for
			// dedicated and global resources.
			if m := instanceTfFileRegex.FindStringSubmatch(info.Name()); len(m) == 4 && m[3] == "" {
				buff, err := ioutil.ReadFile(filepath.Join(p.Dir, info.Name()))
				if err != nil {
					log.Warningln("Cannot parse:", err)
					return err
				}
				tf := TFormat{}
				if err = types.AnyBytes(buff).Decode(&tf); err != nil {
					return err
				}
				for resType, resNameProps := range tf.Resource {
					for resName := range resNameProps {
						if _, has := tfInstFiles[resType]; !has {
							tfInstFiles[resType] = map[TResourceName]string{}
						}
						tfInstFiles[resType][resName] = info.Name()
						log.Debugf("File %s contains resource %s.%s", info.Name(), resType, resName)
					}
				}
			} else if m := tfFileRegex.FindStringSubmatch(info.Name()); len(m) == 3 && m[2] == ".new" {
				tfNewFiles[info.Name()] = struct{}{}
			}
			return nil
		},
	)
	if err != nil {
		return err
	}

	// Determine files to prune, since multiple resource types can exist per file we want to
	// track unique filenames
	prunes := make(map[string]struct{})
	for resType, resNameFilenameMap := range tfInstFiles {
		log.Infof("Detected %v tf.json files for resource type %v", len(resNameFilenameMap), resType)
		if tfStateResNames, has := tfStateResources[resType]; has {
			// State files have instances of this type, check each resource name
			for resName, filename := range resNameFilenameMap {
				if _, has = tfStateResNames[resName]; has {
					log.Infof("Instance %v.%v exists in terraform state", resType, resName)
				} else {
					log.Infof("Detected instance %v.%v to prune at file: %v", resType, resName, filename)
					prunes[filename] = struct{}{}
				}
			}
		} else {
			// No instances of this type in the state file, all should be removed
			log.Infof("State files has no resources of type %v, pruning all %v instances ...", resType, len(resNameFilenameMap))
			for resName, filename := range resNameFilenameMap {
				log.Infof("Detected instance %v.%v to prune at file: %v", resType, resName, filename)
				prunes[filename] = struct{}{}
			}
		}
	}

	log.Infof("Pruning %v tf.json files", len(prunes))
	for filename := range prunes {
		path := filepath.Join(p.Dir, filename)
		err = p.fs.Remove(path)
		if err != nil {
			return err
		}
	}

	// Move any tf.json.new files
	if len(tfNewFiles) == 0 {
		log.Infof("No tf.json.new files to move")
	} else {
		for file := range tfNewFiles {
			path := filepath.Join(p.Dir, file)
			log.Infof("Removing .new suffix from file: %v", path)
			err = p.fs.Rename(path, strings.Replace(path, "tf.json.new", "tf.json", -1))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// doTerraformStateList shells out to run `terraform state list` and parses the result
func (p *plugin) doTerraformStateList() (map[TResourceType]map[TResourceName]struct{}, error) {
	result := map[TResourceType]map[TResourceName]struct{}{}
	command := exec.Command("terraform state list -no-color").
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	command.StartWithHandlers(
		nil,
		func(r io.Reader) error {
			reader := bufio.NewReader(r)
			for {
				lineBytes, _, err := reader.ReadLine()
				if err != nil {
					break
				}
				line := string(lineBytes)
				log.Debugf("'terraform state list' output: %v", line)
				// Every line should have <resource-type>.<resource-name>
				if !strings.Contains(line, ".") {
					log.Errorf("Invalid line from 'terraform state list': %v", line)
					continue
				}
				split := strings.Split(strings.TrimSpace(line), ".")
				resType := TResourceType(split[0])
				resName := TResourceName(split[1])
				if resourceMap, has := result[resType]; has {
					resourceMap[resName] = struct{}{}
				} else {
					result[resType] = map[TResourceName]struct{}{resName: {}}
				}
			}
			return nil
		},
		nil)

	err := command.Wait()
	return result, err
}
