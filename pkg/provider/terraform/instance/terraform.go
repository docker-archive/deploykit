package instance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/docker/infrakit/pkg/util/exec"
	"github.com/spf13/afero"
)

type tf interface {
	doTerraformStateList() (map[TResourceType]map[TResourceName]struct{}, error)
	doTerraformRefresh() error
	doTerraformApply() error
	doTerraformShow([]TResourceType, []string) (result map[TResourceType]map[TResourceName]TResourceProperties, err error)
	doTerraformShowForInstance(string) (result TResourceProperties, err error)
	doTerraformImport(afero.Fs, TResourceType, string, string, bool) error
	doTerraformStateRemove(TResourceType, string) error
}

// terraformLookup returns a tf struct based on the currently installed version of terraform
func terraformLookup(dir string, envs []string) (tf, error) {
	command := exec.Command("terraform version").
		InheritEnvs(true).
		WithEnvs(envs...).
		WithDir(dir)
	versionRegex := regexp.MustCompile("^Terraform v([0-9]*).([0-9]*).([0-9]*)$")
	major, minor := -1, -1
	command.StartWithHandlers(
		nil,
		func(r io.Reader) error {
			reader := bufio.NewReader(r)
			lines := []string{}
			for {
				lineBytes, _, err := reader.ReadLine()
				if err != nil {
					break
				}
				line := string(lineBytes)
				lines = append(lines, line)
				if m := versionRegex.FindAllStringSubmatch(line, -1); len(m) > 0 {
					logger.Info("terraformLookup", "Version", line)
					if v, err := strconv.Atoi(m[0][1]); err == nil {
						major = v
					} else {
						logger.Error("terraformLookup", "Failed to parse major version", "value", m[0][1], "error", err)
					}
					if v, err := strconv.Atoi(m[0][2]); err == nil {
						minor = v
					} else {
						logger.Error("terraformLookup", "Failed to parse minor version", "value", m[0][2], "error", err)
					}
				}
			}
			if major == -1 || minor == -1 {
				logger.Error("Failed to determine terraform version", "output", lines)
			}
			return nil
		},
		nil)
	if err := command.Wait(); err != nil {
		return nil, err
	}
	// Unable to retrieve the version
	if major == -1 || minor == -1 {
		return nil, fmt.Errorf("Unable to determine terraform version")
	}
	// Only support v0.9+
	if major != 0 {
		return nil, fmt.Errorf("Unsupported major version: %d", major)
	}
	// Version 0.9.x
	if minor == 9 {
		return &terraformBase{dir: dir, envs: envs}, nil
	}
	// Version 0.10.x and above
	if minor >= 10 {
		return &terraformV10{
			terraformBase: terraformBase{dir: dir, envs: envs},
			initLock:      sync.Mutex{},
			initCompleted: false,
		}, nil
	}
	return nil, fmt.Errorf("Unsupported minor version: %d", minor)
}

// terraformBase is base implementation and is compatible with terravorm v0.9.x
type terraformBase struct {
	envs []string
	dir  string
}

// terraformV10 is the implementation that is compatible with terraform v.0.10.x+
type terraformV10 struct {
	terraformBase
	initLock      sync.Mutex
	initCompleted bool
}

// doTerraformStateList shells out to run `terraform state list` and parses the result
func (tf *terraformBase) doTerraformStateList() (map[TResourceType]map[TResourceName]struct{}, error) {
	result := map[TResourceType]map[TResourceName]struct{}{}
	command := exec.Command("terraform state list -no-color").
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
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
				logger.Debug("doTerraformStateList", "output", line, "V", debugV3)
				// Every line should have <resource-type>.<resource-name>
				if !strings.Contains(line, ".") {
					logger.Error("doTerraformStateList", "msg", "Invalid line from 'terraform state list'", "line", line)
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

// doTerraformRefresh executes "terraform refresh"
func (tf *terraformBase) doTerraformRefresh() error {
	logger.Info("doTerraformRefresh")
	command := exec.Command("terraform refresh").
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// doTerraformApply executes "terraform apply".
// Version 0.9.x does not not require additional CLI options.
func (tf *terraformBase) doTerraformApply() error {
	return internalTerraformApply(tf.envs, tf.dir)
}

// doTerraformApply executes "terraform apply"
// Version 0.10.+ requires the -auto-approve=true CLI option.
func (tf *terraformV10) doTerraformApply() error {
	tf.doTerraformInit()
	return internalTerraformApply(tf.envs, tf.dir, "-auto-approve=true")
}

// internalTerraformApply executes terraform apply with the given additional parameters
func internalTerraformApply(envs []string, dir string, params ...string) error {
	logger.Info("doTerraformApply", "msg", "Applying plan")
	c := []string{"terraform", "apply", "-refresh=false", "-input=false"}
	c = append(c, params...)
	command := exec.Command(strings.Join(c, " ")).
		InheritEnvs(true).
		WithEnvs(envs...).
		WithDir(dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// doTerraformShow shells out to run `terraform show` and parses the result
func (tf *terraformBase) doTerraformShow(resTypes []TResourceType,
	propFilter []string) (result map[TResourceType]map[TResourceName]TResourceProperties, err error) {

	command := exec.Command("terraform show -no-color").
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
	command.StartWithHandlers(
		nil,
		func(r io.Reader) error {
			found, err := parseTerraformShowOutput(resTypes, propFilter, r)
			result = found
			return err
		},
		nil)

	err = command.Wait()
	return
}

// doTerraformShowForInstance shells out to run `terraform state show <instance>` and parses the result
func (tf *terraformBase) doTerraformShowForInstance(instance string) (result TResourceProperties, err error) {
	command := exec.Command(fmt.Sprintf("terraform state show %v -no-color", instance)).
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
	command.StartWithHandlers(
		nil,
		func(r io.Reader) error {
			props, err := parseTerraformShowForInstanceOutput(r)
			result = props
			return err
		},
		nil)

	err = command.Wait()
	return
}

// doTerraformImport shells out to run `terraform import`
// Version 0.9.x does not require the input resource file to be created prior to the import.
func (tf *terraformBase) doTerraformImport(fs afero.Fs, resType TResourceType, resName, id string, createDummyFile bool) error {
	return internalTerraformImport(tf.envs, tf.dir, resType, resName, id)
}

// doTerraformImport shells out to run `terraform import`
// Version 0.10.+ requires the input resource file to be created prior to the import.
func (tf *terraformV10) doTerraformImport(fs afero.Fs, resType TResourceType, resName, id string, createDummyFile bool) error {
	// The resource file does not need the actual properties, so just create a dummy file
	// with the minimum data. This file can be immediately removed post-import.
	if createDummyFile {
		tFormat := TFormat{
			Resource: map[TResourceType]map[TResourceName]TResourceProperties{
				resType: {
					TResourceName(resName): {},
				},
			},
		}
		buff, err := json.MarshalIndent(tFormat, "  ", "  ")
		if err != nil {
			return err
		}
		path := filepath.Join(tf.dir, "import-resource.tf.json")
		err = afero.WriteFile(fs, path, buff, 0644)
		if err != nil {
			return err
		}
		defer func() {
			fs.Remove(path)
		}()
	}
	tf.doTerraformInit()
	return internalTerraformImport(tf.envs, tf.dir, resType, resName, id)
}

// internalTerraformImport shells out to run `terraform import`
func internalTerraformImport(envs []string, dir string, resType TResourceType, resName, id string) error {
	logger.Info("internalTerraformImport")
	command := exec.Command(fmt.Sprintf("terraform import %v.%v %s", resType, resName, id)).
		InheritEnvs(true).
		WithEnvs(envs...).
		WithDir(dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// doTerraformStateRemove removes the resource from the terraform state file
func (tf *terraformBase) doTerraformStateRemove(vmType TResourceType, vmName string) error {
	command := exec.Command(fmt.Sprintf("terraform state rm %v.%v", vmType, vmName)).
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// doTerraformInit executes "terraform init" and, if the .terraform directory has been created in
// the working directory, tracks that the initialization has completed
func (tf *terraformV10) doTerraformInit() error {
	// Only execute init once
	if tf.initCompleted {
		return nil
	}
	tf.initLock.Lock()
	defer tf.initLock.Unlock()
	if tf.initCompleted {
		return nil
	}
	logger.Info("doTerraformInit")
	command := exec.Command("terraform init -input=false").
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	err := command.Wait()
	if err != nil {
		return err
	}
	// If there are no config files then nothign was initialized, only mark that init
	// compelte if the .terraform direction was created
	path := filepath.Join(tf.terraformBase.dir, ".terraform")
	_, err = os.Stat(path)
	if err == nil {
		tf.initCompleted = true
		return nil
	}
	logger.Warn("Failed to initalize terraform, .terraform directory was not created",
		"path", path,
		"error", err)
	return nil
}
