package instance

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/infrakit/pkg/util/exec"
)

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
func (p *plugin) doTerraformRefresh() error {
	logger.Info("doTerraformRefresh")
	command := exec.Command("terraform refresh").
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// doTerraformApply executes "terraform apply"
func (p *plugin) doTerraformApply() error {
	logger.Info("doTerraformApply", "msg", "Applying plan")
	command := exec.Command("terraform apply -refresh=false").
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// doTerraformShow shells out to run `terraform show` and parses the result
func (p *plugin) doTerraformShow(resTypes []TResourceType,
	propFilter []string) (result map[TResourceType]map[TResourceName]TResourceProperties, err error) {

	command := exec.Command("terraform show -no-color").
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
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
func (p *plugin) doTerraformShowForInstance(instance string) (result TResourceProperties, err error) {

	command := exec.Command(fmt.Sprintf("terraform state show %v -no-color", instance)).
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
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
func (p *plugin) doTerraformImport(resType TResourceType, resName, id string) error {
	command := exec.Command(fmt.Sprintf("terraform import %v.%v %s", resType, resName, id)).
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}

// cleanupFailedImport removes the resource from the terraform state file
func (p *plugin) cleanupFailedImport(vmType TResourceType, vmName string) error {
	command := exec.Command(fmt.Sprintf("terraform state rm %v.%v", vmType, vmName)).
		InheritEnvs(true).
		WithEnvs(p.envs...).
		WithDir(p.Dir)
	if err := command.WithStdout(os.Stdout).WithStderr(os.Stdout).Start(); err != nil {
		return err
	}
	return command.Wait()
}
