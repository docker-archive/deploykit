package base

import (
	"io/ioutil"
	"os"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/spf13/pflag"
)

// ReadFromStdinIfElse checks condition and reads from stdin if true; otherwise it executes other.
func ReadFromStdinIfElse(
	condition func() bool,
	otherwise func() (string, error),
	toJSON cli.ToJSONFunc) (rendered string, err error) {

	if condition() {
		buff, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		json, err := toJSON(buff)
		log.Debug("stdin", "buffer", string(json))
		return string(json), err
	}
	rendered, err = otherwise()
	if err != nil {
		return
	}
	var buff []byte
	buff, err = toJSON([]byte(rendered))
	if err != nil {
		return
	}
	return string(buff), nil
}

// TemplateProcessor returns a flagset and a function for processing template input.
func TemplateProcessor(plugins func() discovery.Plugins) (*pflag.FlagSet, cli.ToJSONFunc, cli.FromJSONFunc, cli.ProcessTemplateFunc) {
	services := cli.NewServices(plugins)
	return services.ProcessTemplateFlags,
		services.ToJSON,
		services.FromJSON,
		services.ProcessTemplate
}
