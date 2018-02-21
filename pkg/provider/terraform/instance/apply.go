package instance

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/deckarep/golang-set"
	manager_discovery "github.com/docker/infrakit/pkg/manager/discovery"
	ibmcloud_client "github.com/docker/infrakit/pkg/provider/ibmcloud/client"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/afero"
)

// TResourceFilenameProps contains the filename and the associated properties
// for a specific resource
type TResourceFilenameProps struct {
	FileName  string
	FileProps TResourceProperties
}

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
			logger.Info("terraformApply", "msg", "Successfully interrupted terraform apply goroutine")
		default:
			logger.Info("terraformApply", "msg", "Polling channel is full, not interrupting")
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
					getExistingResource: p.getExistingResource,
				}
				// The trigger for an apply is typically from a group commit, sleep for a few seconds so
				// that multiple .tf.json.new files have time to be created
				if initial {
					time.Sleep(time.Second * 5)
					// And only run if there have been no file deltas in the last few seconds, the delta
					// processing ignores files that are more then 30 seconds in the future so this
					// should never wait indefinately but, to be safe, only wait for no deltas for at most
					// 30 seconds
					for i := 0; i < 30; i++ {
						hasDelta, err := p.hasRecentDeltas(3)
						if hasDelta {
							time.Sleep(time.Second * 1)
							continue
						}
						if err != nil {
							logger.Error("terraformApply", "msg", "Failed to determine file deltas", "error", err)
						}
						break
					}
				}
				if err := p.handleFiles(fns); err == nil {
					if err = p.terraform.doTerraformApply(); err == nil {
						// Goroutine was interrupted, this likely means that there was a file change; now that
						// apply is finished we want to clear the cache since we expect a delta
						if initial {
							p.fsLock.Lock()
							p.clearCachedInstances()
							p.fsLock.Unlock()
							initial = false
						}
					} else {
						logger.Error("terraformApply", "msg", "Failed to execute 'terraform apply'", "error", err)
					}
				} else {
					logger.Error("terraformApply", "msg", "Not executing 'terraform apply'", "error", err)
				}
			} else {
				logger.Info("terraformApply", "msg", fmt.Sprintf("Not applying terraform, checking again in %v", p.pollInterval))
				// Either not running on the leader or failed to determine leader, clear
				// cache since another leader may be altering the instances
				p.fsLock.Lock()
				p.clearCachedInstances()
				p.fsLock.Unlock()
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
		logger.Error("shouldApply", "msg", "Failed to locate manager plugin", "error", err)
		return false
	}
	isLeader, err := manager.IsLeader()
	if err != nil {
		logger.Error("shouldApply", "msg", "Failed to determine manager leadership", "error", err)
		return false
	}
	if isLeader {
		logger.Debug("shouldApply", "msg", "Running on leader manager, applying terraform", "V", debugV1)
		return true
	}
	logger.Info("shouldApply", "msg", "Not running on leader manager, not applying terraform")
	return false
}

// External functions to use during when pruning files; broken out for testing
type tfFuncs struct {
	getExistingResource func(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error)
}

// hasRecentDeltas returns true if any tf.json[.new] files have been changed in
// in the last "window" seconds
func (p *plugin) hasRecentDeltas(window int) (bool, error) {
	p.fsLock.RLock()
	defer p.fsLock.RUnlock()

	now := time.Now()
	modTime := time.Time{}
	fs := &afero.Afero{Fs: p.fs}
	err := fs.Walk(p.Dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// If the file has been removed just ignore it
					logger.Debug("hasRecentDeltas", "msg", fmt.Sprintf("Ignoring file %s", path), "error", err, "V", debugV3)
					return nil
				}
				logger.Error("hasRecentDeltas", "msg", fmt.Sprintf("Failed to process file %s", path), "error", err)
				return err
			}
			if m := tfFileRegex.FindStringSubmatch(info.Name()); len(m) == 3 {
				if info.ModTime().After(now) {
					// The file timestamp is in the future, this is fine if we are within 30 seconds but
					// it should be ignored if it's further out (if there is a file with a timestamp
					// that's a full day ahead we'd never process terraform until the local time catches
					// up -- this should never happen but we should handle it)
					if info.ModTime().After(now.Add(time.Duration(30) * time.Second)) {
						logger.Error("hasRecentDeltas",
							"msg",
							fmt.Sprintf("Terraform file %v has been updated in the future, ignoring timestamp in delta check (delta=%v)",
								info.Name(),
								now.Sub(info.ModTime())))
						return nil
					}
				}
				if modTime.Before(info.ModTime()) {
					modTime = info.ModTime()
				}
			}
			return nil
		},
	)
	if err != nil {
		return false, err
	}
	if !modTime.IsZero() {
		if modTime.After(now.Add(-(time.Duration(window) * time.Second))) {
			logger.Info("hasRecentDeltas",
				"msg",
				fmt.Sprintf("Terraform file updates are within %v seconds (delta=%v)", window, now.Sub(modTime)))
			return true, nil
		}
		logger.Info("hasRecentDeltas",
			"msg",
			fmt.Sprintf("Terraform file updates are outside %v seconds (delta=%v)", window, now.Sub(modTime)))
	}
	return false, nil
}

// handleFiles handles resource pruning and new resources via:
// 1. Cache resource types/names from terraform state file
// 2. Execute "terraform refresh" to refresh state
// 3. Acquire file system lock
// 4. Identity ".tf.json" files that are not in the terraform state and, for each:
// 4.1 If the resource was previously in the state file (from #2) then prune
// 4.2 Else, query the backend cloud to see if the resource exists and was missing from the
//     state file (this can happen if a manager failover occurs during a provision) and either
//     import (if found) or prune
// 5. Remove all "tf.json.new" files to "tf.json"
//
// Once these steps are done then "terraform apply" can execute without the
// file system lock.
func (p *plugin) handleFiles(fns tfFuncs) error {
	// TODO(kaufers): If it possible that not all of the .new files were moved to
	//  .tf.json files (NFS connection could be lost) and this could make the refresh
	//  always fail due to references that are not valid. Update this flow to still
	//  rename .new files even if the refresh fails (but do not prune or apply since
	//  we need valid refresh'd data) and then let the next iteration attempt to
	//  reconcile things.

	// Get the current resources, this must happen before a refresh so that we can
	// identity orphans from an incomplete "apply"
	tfStateResourcesBefore, err := p.terraform.doTerraformStateList()
	if err != nil {
		return err
	}

	// Refresh all resources, anything deleted from the backend will be removed
	// from the state file
	if err = p.terraform.doTerraformRefresh(); err != nil {
		return err
	}

	// And now get the updated resources
	tfStateResourcesAfter, err := p.terraform.doTerraformStateList()
	if err != nil {
		return err
	}

	// Once we have the update resources we need to lock out any new files (from Provision)
	// and the listing of the files (from Describe) while we reconcile orphans and rename
	p.fsLock.Lock()
	defer func() {
		p.clearCachedInstances()
		p.fsLock.Unlock()
	}()

	// Load all instance files and all new files from disk
	tfInstFiles := map[TResourceType]map[TResourceName]TResourceFilenameProps{}
	tfNewFiles := map[string]struct{}{}
	fs := &afero.Afero{Fs: p.fs}
	err = fs.Walk(p.Dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// If the file has been removed just ignore it
					logger.Debug("handleFiles", "msg", fmt.Sprintf("Ignoring file %s", path), "error", err, "V", debugV3)
					return nil
				}
				logger.Error("handleFiles", "msg", fmt.Sprintf("Failed to process file %s", path), "error", err)
				return err
			}
			// Only the VM files are valid for pruning; once pruned then the group controller polling will
			// ensure that a replacement is created. There is no mechanism that ensures consistency for
			// dedicated and global resources.
			if m := instanceTfFileRegex.FindStringSubmatch(info.Name()); len(m) == 4 && m[3] == "" {
				buff, err := ioutil.ReadFile(filepath.Join(p.Dir, info.Name()))
				if err != nil {
					if os.IsNotExist(err) {
						logger.Debug("handleFiles", "msg", fmt.Sprintf("Ignoring removed file %s", path), "error", err)
						return nil
					}
					logger.Warn("handleFiles", "msg", fmt.Sprintf("Cannot read file %s", path))
					return err
				}
				tf := TFormat{}
				if err = types.AnyBytes(buff).Decode(&tf); err != nil {
					return err
				}
				for resType, resNameProps := range tf.Resource {
					for resName, resProps := range resNameProps {
						if _, has := tfInstFiles[resType]; !has {
							tfInstFiles[resType] = map[TResourceName]TResourceFilenameProps{}
						}
						addToResTypeNamePropsMap(tfInstFiles, resType, resName, info.Name(), resProps)
						logger.Debug("handleFiles", "msg", fmt.Sprintf("File %s contains resource %s.%s", info.Name(), resType, resName), "V", debugV1)
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

	// Handle orphan resources and file pruning
	prunes := make(map[TResourceType]map[TResourceName]TResourceFilenameProps)
	for resType, resNameFilenamePropsMap := range tfInstFiles {
		logger.Info("handleFiles", "msg", fmt.Sprintf("Detected %v tf.json files for resource type %v", len(resNameFilenamePropsMap), resType))
		if tfStateResName, has := tfStateResourcesAfter[resType]; has {
			// State files have instances of this type, check each resource name
			for resName, propsFilename := range resNameFilenamePropsMap {
				if _, has = tfStateResName[resName]; has {
					logger.Debug("handleFiles", "msg", fmt.Sprintf("Instance %v.%v exists in terraform state", resType, resName), "V", debugV1)
				} else {
					logger.Info("handleFiles", "msg", fmt.Sprintf("Detected candidate instance %v.%v to prune at file: %v", resType, resName, propsFilename.FileName))
					addToResTypeNamePropsMap(prunes, resType, resName, propsFilename.FileName, propsFilename.FileProps)
				}
			}
		} else {
			// No instances of this type in the state file, all should be removed
			logger.Info("handleFiles", "msg", fmt.Sprintf("State files has no resources of type %v, pruning all %v instances ...", resType, len(resNameFilenamePropsMap)))
			for resName, propsFilename := range resNameFilenamePropsMap {
				logger.Info("handleFiles", "msg", fmt.Sprintf("Detected candidate instance %v.%v to prune at file: %v", resType, resName, propsFilename.FileName))
				addToResTypeNamePropsMap(prunes, resType, resName, propsFilename.FileName, propsFilename.FileProps)
			}
		}
	}
	if err = p.handleFilePruning(fns, prunes, tfStateResourcesBefore); err != nil {
		return err
	}

	// Move any tf.json.new files
	if len(tfNewFiles) == 0 {
		logger.Info("handleFiles", "msg", "No tf.json.new files to move")
	} else {
		for file := range tfNewFiles {
			path := filepath.Join(p.Dir, file)
			logger.Info("handleFiles", "msg", fmt.Sprintf("Removing .new suffix from file: %v", path))
			err = p.fs.Rename(path, strings.Replace(path, "tf.json.new", "tf.json", -1))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// addToResTypeNamePropsMap is a utility function to populate the given map with
// the given data
func addToResTypeNamePropsMap(
	m map[TResourceType]map[TResourceName]TResourceFilenameProps,
	resType TResourceType,
	resName TResourceName,
	filename string,
	props TResourceProperties,
) {
	resNameFileProps, has := m[resType]
	if !has {
		resNameFileProps = make(map[TResourceName]TResourceFilenameProps)
		m[resType] = resNameFileProps
	}
	resNameFileProps[resName] = TResourceFilenameProps{
		FileName:  filename,
		FileProps: props,
	}
}

// handleFilePruning processes the prune file that are no longer associated with resources
// in terraform. If the resource was not in terraform state before the refresh then
// the backend cloud is queried in order to determine if it still exists; if it does then
// it is imported and, if it does not, then the file is pruned.
func (p *plugin) handleFilePruning(
	fns tfFuncs,
	prunes map[TResourceType]map[TResourceName]TResourceFilenameProps,
	tfStateBeforeRefresh map[TResourceType]map[TResourceName]struct{},
) error {
	// Determine files to prune, since multiple resource types can exist per file we want to
	// track unique filenames
	pruneFiles := make(map[string]struct{})
	// If the resource was removed out-of-band then it had a previous entry in the state file
	// and can be pruned; if there is no entry then query the backend to determine if the
	// resource still exists
	for resType, resNameFilenameProps := range prunes {
		var tfResTypeNameProps map[TResourceName]struct{}
		tfResTypeNameProps, has := tfStateBeforeRefresh[resType]
		if !has {
			tfResTypeNameProps = make(map[TResourceName]struct{})
		}
		for resName, resFilenameProps := range resNameFilenameProps {
			if _, has := tfResTypeNameProps[resName]; has {
				logger.Info("handleFilePruning",
					"msg",
					fmt.Sprintf("Pruning %v file, resource %v.%v previously existed in terraform",
						resFilenameProps.FileName,
						resType,
						resName))
				pruneFiles[resFilenameProps.FileName] = struct{}{}
			} else {
				// Find resource type in backend
				importID, err := fns.getExistingResource(resType, resName, resFilenameProps.FileProps)
				if err != nil {
					return err
				}
				// No ID returned, prune file
				if importID == nil {
					logger.Info("handleFilePruning",
						"msg",
						fmt.Sprintf("Pruning %v file, resource %v.%v was not found in backend",
							resFilenameProps.FileName,
							resType,
							resName))
					pruneFiles[resFilenameProps.FileName] = struct{}{}
				} else {
					// Import resource. Note that the input tf.json file is already on disk.
					logger.Info("handleFilePruning",
						"msg",
						fmt.Sprintf("Importing %v %v into terraform as resource %v ...",
							string(resType),
							*importID,
							string(resName)))
					if err = p.terraform.doTerraformImport(p.fs, resType, string(resName), *importID, false); err != nil {
						return err
					}
				}
			}
		}
	}
	if len(pruneFiles) > 0 {
		logger.Info("handleFilePruning", "msg", fmt.Sprintf("Pruning %v tf.json files", len(pruneFiles)))
		for file := range pruneFiles {
			path := filepath.Join(p.Dir, file)
			logger.Info("handleFilePruning", "msg", fmt.Sprintf("Pruning file: %v", file))
			err := p.fs.Remove(path)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// getExistingResource queries the backend cloud to get the ID of the resource associated
// with the given type, name, and properties
func (p *plugin) getExistingResource(resType TResourceType, resName TResourceName, props TResourceProperties) (*string, error) {
	// Ony VMs retrival is supported
	supportedVMs := mapset.NewSetFromSlice(VMTypes)
	if !supportedVMs.Contains(resType) {
		return nil, nil
	}
	switch resType {
	case VMSoftLayer, VMIBMCloud:
		tagsProp, has := props["tags"]
		if !has {
			return nil, nil
		}
		// Convert tags to String
		tagsInterface, ok := tagsProp.([]interface{})
		if !ok {
			return nil, fmt.Errorf("Cannot process tags, unknown type: %v", reflect.TypeOf(tagsProp))
		}
		tags := make([]string, len(tagsInterface))
		for i, t := range tagsInterface {
			tags[i] = fmt.Sprintf("%v", t)
		}
		// Creds either in env vars or in the plugin Env slice
		username := os.Getenv(SoftlayerUsernameEnvVar)
		apiKey := os.Getenv(SoftlayerAPIKeyEnvVar)
		if username == "" || apiKey == "" {
			for _, env := range p.envs {
				if !strings.Contains(env, "=") {
					continue
				}
				split := strings.Split(env, "=")
				switch split[0] {
				case SoftlayerUsernameEnvVar:
					username = split[1]
				case SoftlayerAPIKeyEnvVar:
					apiKey = split[1]
				}
			}
		}
		id, err := GetIBMCloudVMByTag(ibmcloud_client.GetClient(username, apiKey), tags)
		if err != nil {
			return nil, err
		}
		if id == nil {
			return nil, nil
		}
		idString := strconv.Itoa(*id)
		return &idString, nil
	}
	logger.Warn("getExistingResource", "msg", fmt.Sprintf("Unsupported VM type for backend retrival: %v", resType))
	return nil, nil
}
