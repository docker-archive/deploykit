package instance

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/pflag"
)

// View is a view of an Instance
type View struct {
	options            template.Options
	quiet              bool
	tags               []string // tags to filter
	properties         bool
	tagsTemplate       string
	propertiesTemplate string
}

// FlagSet returns a flagset to bind
func (v *View) FlagSet() *pflag.FlagSet {
	// defaults
	v.tags = []string{}
	v.tagsTemplate = "*"
	v.propertiesTemplate = "{{.}}"

	fs := pflag.NewFlagSet("instance-view", pflag.ExitOnError)
	fs.StringSliceVar(&v.tags, "tags", v.tags, "Tags to filter")
	fs.BoolVarP(&v.properties, "properties", "p", v.properties, "True to show properties")
	fs.StringVarP(&v.tagsTemplate, "tags-view", "t", v.tagsTemplate, "Template for rendering tags")
	fs.StringVarP(&v.propertiesTemplate, "properties-view", "w", v.propertiesTemplate, "Template for rendering properties")
	fs.BoolVarP(&v.quiet, "quiet", "q", v.quiet, "Print rows without column headers")
	return fs
}

// ShowProperties returns true if showing properties
func (v *View) ShowProperties() bool {
	return v.properties
}

// TagFilter returns the tag filter for querying
func (v *View) TagFilter() map[string]string {
	filter := map[string]string{}
	for _, t := range v.tags {
		p := strings.Split(t, "=")
		if len(p) == 2 {
			filter[p[0]] = p[1]
		} else {
			filter[p[0]] = ""
		}
	}
	return filter
}

// Renderer returns the renderer for the results
func (v *View) Renderer() (func(w io.Writer, v interface{}) error, error) {
	tagsView, err := template.NewTemplate(template.ValidURL(v.tagsTemplate), v.options)
	if err != nil {
		return nil, err
	}
	propertiesView, err := template.NewTemplate(template.ValidURL(v.propertiesTemplate), v.options)
	if err != nil {
		return nil, err
	}

	return func(w io.Writer, result interface{}) error {

		instances, is := result.([]instance.Description)
		if !is {
			return fmt.Errorf("not []instance.Description")
		}

		if !v.quiet {
			if v.properties {
				fmt.Printf("%-30s\t%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS", "PROPERTIES")

			} else {
				fmt.Printf("%-30s\t%-30s\t%-s\n", "ID", "LOGICAL", "TAGS")
			}
		}
		for _, d := range instances {

			logical := "  -   "
			if d.LogicalID != nil {
				logical = string(*d.LogicalID)
			}

			tagViewBuff := ""
			if v.tagsTemplate == "*" {
				// default -- this is a hack
				printTags := []string{}
				for k, v := range d.Tags {
					printTags = append(printTags, fmt.Sprintf("%s=%s", k, v))
				}
				sort.Strings(printTags)
				tagViewBuff = strings.Join(printTags, ",")
			} else {
				tagViewBuff = renderTags(d.Tags, tagsView)
			}

			if v.properties {
				fmt.Printf("%-30s\t%-30s\t%-30s\t%-s\n", d.ID, logical, tagViewBuff,
					renderProperties(d.Properties, propertiesView))
			} else {
				fmt.Printf("%-30s\t%-30s\t%-s\n", d.ID, logical, tagViewBuff)
			}
		}
		return nil
	}, nil
}

func renderTags(m map[string]string, view *template.Template) string {
	buff, err := view.Render(m)
	if err != nil {
		return err.Error()
	}
	return buff
}

func renderProperties(properties *types.Any, view *template.Template) string {
	var v interface{}
	err := properties.Decode(&v)
	if err != nil {
		return err.Error()
	}

	buff, err := view.Render(v)
	if err != nil {
		return err.Error()
	}
	return buff
}
