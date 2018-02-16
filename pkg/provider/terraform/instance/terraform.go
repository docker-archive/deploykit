package instance

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/infrakit/pkg/util/exec"
)

type tf interface {
	doTerraformStateList() (map[TResourceType]map[TResourceName]struct{}, error)
	doTerraformRefresh() error
	doTerraformApply() error
	doTerraformShow([]TResourceType, []string) (result map[TResourceType]map[TResourceName]TResourceProperties, err error)
	doTerraformShowForInstance(string) (result TResourceProperties, err error)
	doTerraformImport(TResourceType, string, string) error
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
	if minor >= 9 {
		return &terraformBase{dir: dir, envs: envs}, nil
	}
	return nil, fmt.Errorf("Unsupported minor version: %d", minor)
}

// terraformBase is base implementation and is compatible with terravorm v0.9.x
type terraformBase struct {
	envs []string
	dir  string
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

// doTerraformApply executes "terraform refresh"
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

// doTerraformApply executes "terraform apply"
func (tf *terraformBase) doTerraformApply() error {
	logger.Info("doTerraformApply", "msg", "Applying plan")
	command := exec.Command("terraform apply -refresh=false").
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
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
func (tf *terraformBase) doTerraformImport(resType TResourceType, resName, id string) error {
	command := exec.Command(fmt.Sprintf("terraform import %v.%v %s", resType, resName, id)).
		InheritEnvs(true).
		WithEnvs(tf.envs...).
		WithDir(tf.dir)
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
