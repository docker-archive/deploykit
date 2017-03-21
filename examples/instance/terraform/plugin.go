package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/nightlyone/lockfile"
	"github.com/spf13/afero"
)

// This example uses terraform as the instance plugin.
// It is very similar to the file instance plugin.  When we
// provision an instance, we write a *.tf.json file in the directory
// and call terra apply.  For describing instances, we parse the
// result of terra show.  Destroying an instance is simply removing a
// tf.json file and call terra apply again.

type plugin struct {
	Dir       string
	fs        afero.Fs
	lock      lockfile.Lockfile
	applying  bool
	applyLock sync.Mutex
	pretend   bool // true to actually do terraform apply
}

// NewTerraformInstancePlugin returns an instance plugin backed by disk files.
func NewTerraformInstancePlugin(dir string) instance.Plugin {
	log.Debugln("terraform instance plugin. dir=", dir)
	lock, err := lockfile.New(filepath.Join(dir, "tf-apply.lck"))
	if err != nil {
		panic(err)
	}

	return &plugin{
		Dir:  dir,
		fs:   afero.NewOsFs(),
		lock: lock,
	}
}

/*
TFormat models the on disk representation of a terraform resource JSON.

An example of this looks like:

{
    "resource" : {
	"aws_instance" : {
	    "web4" : {
		"ami" : "${lookup(var.aws_amis, var.aws_region)}",
		"instance_type" : "m1.small",
		"key_name": "PUBKEY",
		"vpc_security_group_ids" : ["${aws_security_group.default.id}"],
		"subnet_id": "${aws_subnet.default.id}",
		"tags" :  {
		    "Name" : "web4",
		    "InstancePlugin" : "terraform"
		}
		"connection" : {
		    "user" : "ubuntu"
		},
		"provisioner" : {
		    "remote_exec" : {
			"inline" : [
			    "sudo apt-get -y update",
			    "sudo apt-get -y install nginx",
			    "sudo service nginx start"
			]
		    }
		}
	    }
	}
    }
}

Note that the JSON above has a name (web4).  In general, we do not require names to
be specified. So this means the raw JSON we support needs to omit the name. So the instance.Spec
JSON looks like below, where the value of `value` is the instance body of the TF format JSON.

{
    "Properties" : {
        "type" : "aws_instance",
        "value" : {
            "ami" : "${lookup(var.aws_amis, var.aws_region)}",
            "instance_type" : "m1.small",
            "key_name": "PUBKEY",
            "vpc_security_group_ids" : ["${aws_security_group.default.id}"],
            "subnet_id": "${aws_subnet.default.id}",
            "tags" :  {
                "Name" : "web4",
                "InstancePlugin" : "terraform"
            },
            "connection" : {
                "user" : "ubuntu"
            },
            "provisioner" : {
                "remote_exec" : {
                    "inline" : [
                        "sudo apt-get -y update",
                        "sudo apt-get -y install nginx",
                        "sudo service nginx start"
                    ]
                }
            }
        }
    },
    "Tags" : {
        "other" : "values",
        "to" : "merge",
        "with" : "tags"
    },
    "Init" : "init string"
}

*/
type TFormat struct {

	// Resource : resource_type : name : map[string]interface{}
	Resource map[string]map[string]map[string]interface{} `json:"resource"`
}

// SpecPropertiesFormat is the schema in the Properties field of the instance.Spec JSON
type SpecPropertiesFormat struct {
	Type  string                 `json:"type"`
	Value map[string]interface{} `json:"value"`
}

// Validate performs local validation on a provision request.
func (p *plugin) Validate(req *types.Any) error {
	log.Debugln("validate", req.String())

	parsed := SpecPropertiesFormat{}
	err := req.Decode(&parsed)
	if err != nil {
		return err
	}

	if parsed.Type == "" {
		return fmt.Errorf("no-resource-type:%s", req.String())
	}

	if len(parsed.Value) == 0 {
		return fmt.Errorf("no-value:%s", req.String())
	}
	return nil
}

func addUserData(m map[string]interface{}, key string, init string) {
	if v, has := m[key]; has {
		m[key] = fmt.Sprintf("%s\n%s", v, init)
	} else {
		m[key] = init
	}
}

func (p *plugin) terraformApply() error {
	if p.pretend {
		return nil
	}

	p.applyLock.Lock()
	defer p.applyLock.Unlock()

	if p.applying {
		return nil
	}

	go func() {
		for {
			if err := p.lock.TryLock(); err == nil {
				defer p.lock.Unlock()
				doTerraformApply(p.Dir)
			}
			log.Debugln("Can't acquire lock, waiting")
			time.Sleep(time.Duration(int64(rand.NormFloat64())%1000) * time.Millisecond)
		}
	}()
	p.applying = true
	return nil
}

func doTerraformApply(dir string) error {
	log.Infoln(time.Now().Format(time.RFC850) + " Applying plan")
	cmd := exec.Command("terraform", "apply")
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	output := io.MultiReader(stdout, stderr)
	go func() {
		reader := bufio.NewReader(output)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			log.WithField("terraform", "apply").Infoln(line)
		}
	}()
	return cmd.Run() // blocks
}

func (p *plugin) terraformShow() (map[string]interface{}, error) {
	re := regexp.MustCompile("(^instance-[0-9]+)(.tf.json)")

	result := map[string]interface{}{}

	fs := &afero.Afero{Fs: p.fs}
	// just scan the directory for the instance-*.tf.json files
	err := fs.Walk(p.Dir, func(path string, info os.FileInfo, err error) error {
		matches := re.FindStringSubmatch(info.Name())

		if len(matches) == 3 {
			id := matches[1]
			parse := map[string]interface{}{}

			buff, err := ioutil.ReadFile(filepath.Join(p.Dir, info.Name()))

			if err != nil {
				log.Warningln("Cannot parse:", err)
				return err
			}

			err = json.Unmarshal(buff, &parse)
			if err != nil {
				return err
			}

			if res, has := parse["resource"].(map[string]interface{}); has {
				var first map[string]interface{}
			res:
				for _, r := range res {
					if f, ok := r.(map[string]interface{}); ok {
						first = f
						break res
					}
				}
				if props, has := first[id]; has {
					result[id] = props
				}
			}
		}
		return nil
	})
	return result, err
}

func (p *plugin) parseTfStateFile() (map[string]interface{}, error) {
	// open the terraform.tfstate file
	buff, err := ioutil.ReadFile(filepath.Join(p.Dir, "terraform.tfstate"))
	if err != nil {

		// The tfstate file is not present this means we have to apply it first.
		if os.IsNotExist(err) {
			if err = p.terraformApply(); err != nil {
				return nil, err
			}
			return p.terraformShow()
		}
		return nil, err
	}

	// tfstate is a JSON so query it
	parsed := map[string]interface{}{}
	err = json.Unmarshal(buff, &parsed)
	if err != nil {
		return nil, err
	}

	if m1, has := parsed["modules"].([]interface{}); has && len(m1) > 0 {
		module := m1[0]
		if mm, ok := module.(map[string]interface{}); ok {
			if resources, ok := mm["resources"].(map[string]interface{}); ok {

				// the attributes are wrapped under each resource objects'
				// primary.attributes
				result := map[string]interface{}{}
				for k, rr := range resources {
					if r, ok := rr.(map[string]interface{}); ok {
						if primary, ok := r["primary"].(map[string]interface{}); ok {
							if attributes, ok := primary["attributes"]; ok {
								result[k] = attributes
							}
						}
					}
				}
				return result, nil
			}
		}
	}
	return nil, nil
}

func (p *plugin) ensureUniqueFile() string {
	for {
		if err := p.lock.TryLock(); err == nil {
			defer p.lock.Unlock()
			return ensureUniqueFile(p.Dir)
		}
		log.Infoln("Can't acquire lock, waiting")
		time.Sleep(time.Duration(int64(rand.NormFloat64())%1000) * time.Millisecond)
	}
}

func ensureUniqueFile(dir string) string {
	n := fmt.Sprintf("instance-%d", time.Now().Unix())
	// if we can open then we have to try again...  the file cannot exist currently
	if f, err := os.Open(filepath.Join(dir, n) + ".tf.json"); err == nil {
		f.Close()
		return ensureUniqueFile(dir)
	}
	return n
}

// Provision creates a new instance based on the spec.
func (p *plugin) Provision(spec instance.Spec) (*instance.ID, error) {
	// Simply writes a file and call terraform apply

	if spec.Properties == nil {
		return nil, fmt.Errorf("no-properties")
	}

	properties := SpecPropertiesFormat{}
	err := spec.Properties.Decode(&properties)
	if err != nil {
		return nil, err
	}

	// use timestamp as instance id
	name := p.ensureUniqueFile()

	id := instance.ID(name)

	// set the tags.
	// add a name
	if spec.Tags != nil {
		if _, has := spec.Tags["Name"]; !has {
			spec.Tags["Name"] = string(id)
		}
	}

	// Use the given hostname value as a prefix if it is a non-empty string
	if hostnamePrefix, is := properties.Value["@hostname_prefix"].(string); is {
		hostnamePrefix = strings.Trim(hostnamePrefix, " ")
		// Use the default behavior if hostnamePrefix was either not a string, or an empty string
		if hostnamePrefix == "" {
			properties.Value["hostname"] = name
		} else {
			// Remove "instance-" from "instance-XXXX", then append that string to the hostnamePrefix to create the new hostname
			properties.Value["hostname"] = fmt.Sprintf("%s-%s", hostnamePrefix, strings.Replace(name, "instance-", "", -1))
		}
	} else {
		properties.Value["hostname"] = name
	}
	// Delete hostnamePrefix so it will not be written in the *.tf.json file
	delete(properties.Value, "@hostname_prefix")
	log.Debugln("Adding hostname to properties: hostname=", properties.Value["hostname"])

	switch properties.Type {
	case "aws_instance", "azurerm_virtual_machine", "digitalocean_droplet", "google_compute_instance":
		if t, exists := properties.Value["tags"]; !exists {
			properties.Value["tags"] = spec.Tags
		} else if mm, ok := t.(map[string]interface{}); ok {
			// merge tags
			for tt, vv := range spec.Tags {
				mm[tt] = vv
			}
		}
	case "softlayer_virtual_guest":
		if _, has := properties.Value["tags"]; !has {
			properties.Value["tags"] = []interface{}{}
		}
		tags, ok := properties.Value["tags"].([]interface{})
		if ok {
			//softlayer uses a list of tags, instead of a map of tags
			properties.Value["tags"] = mergeLabelsIntoTagSlice(tags, spec.Tags)
		}
	}

	// Use tag to store the logical id
	if spec.LogicalID != nil {
		if m, ok := properties.Value["tags"].(map[string]interface{}); ok {
			m["LogicalID"] = string(*spec.LogicalID)
		}
	}
	switch properties.Type {
	case "aws_instance":
		if p, exists := properties.Value["private_ip"]; exists {
			if p == "INSTANCE_LOGICAL_ID" {
				if spec.LogicalID != nil {
					// set private IP to logical ID
					properties.Value["private_ip"] = string(*spec.LogicalID)
				} else {
					// reset private IP (the tag is not relevant in this context)
					delete(properties.Value, "private_ip")
				}
			}
		}
	}

	// merge the inits
	switch properties.Type {
	case "aws_instance", "digitalocean_droplet":
		addUserData(properties.Value, "user_data", spec.Init)
	case "softlayer_virtual_guest":
		addUserData(properties.Value, "user_metadata", spec.Init)
	case "azurerm_virtual_machine":
		// os_profile.custom_data
		if m, has := properties.Value["os_profile"]; !has {
			properties.Value["os_profile"] = map[string]interface{}{
				"custom_data": spec.Init,
			}
		} else if mm, ok := m.(map[string]interface{}); ok {
			addUserData(mm, "custom_data", spec.Init)
		}
	case "google_compute_instance":
		// metadata_startup_script
		addUserData(properties.Value, "metadata_startup_script", spec.Init)
	}

	tfFile := TFormat{
		Resource: map[string]map[string]map[string]interface{}{
			properties.Type: {
				name: properties.Value,
			},
		},
	}

	buff, err := json.MarshalIndent(tfFile, "  ", "  ")
	log.Debugln("provision", id, "data=", string(buff), "err=", err)
	if err != nil {
		return nil, err
	}

	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, string(id)+".tf.json"), buff, 0644)
	if err != nil {
		return nil, err
	}

	return &id, p.terraformApply()
}

// Label labels the instance
func (p *plugin) Label(instance instance.ID, labels map[string]string) error {
	buff, err := afero.ReadFile(p.fs, filepath.Join(p.Dir, string(instance)+".tf.json"))
	if err != nil {
		return err
	}

	tfFile := map[string]interface{}{}
	err = json.Unmarshal(buff, &tfFile)
	if err != nil {
		return err
	}

	resources, has := tfFile["resource"].(map[string]interface{})
	if !has {
		return fmt.Errorf("bad tfile:%v", instance)
	}

	var resourceType string
	var first map[string]interface{} // there should be only one element keyed by the type (e.g. aws_instance)
	for k, r := range resources {
		if f, ok := r.(map[string]interface{}); ok {
			first = f
			resourceType = k
			break
		}
	}

	if len(first) == 0 {
		return fmt.Errorf("no typed properties:%v", instance)
	}

	props, has := first[string(instance)].(map[string]interface{})
	if !has {
		return fmt.Errorf("not found:%v", instance)
	}

	switch resourceType {
	case "aws_instance", "azurerm_virtual_machine", "digitalocean_droplet", "google_compute_instance":
		if _, has := props["tags"]; !has {
			props["tags"] = map[string]interface{}{}
		}

		if tags, ok := props["tags"].(map[string]interface{}); ok {
			for k, v := range labels {
				tags[k] = v
			}
		}

	case "softlayer_virtual_guest":
		if _, has := props["tags"]; !has {
			props["tags"] = []interface{}{}
		}
		tags, ok := props["tags"].([]interface{})
		if !ok {
			return fmt.Errorf("bad format:%v", instance)
		}
		props["tags"] = mergeLabelsIntoTagSlice(tags, labels)
	}

	buff, err = json.MarshalIndent(tfFile, "  ", "  ")
	if err != nil {
		return err
	}
	err = afero.WriteFile(p.fs, filepath.Join(p.Dir, string(instance)+".tf.json"), buff, 0644)
	if err != nil {
		return err
	}
	return p.terraformApply()
}

// Destroy terminates an existing instance.
func (p *plugin) Destroy(instance instance.ID) error {
	fp := filepath.Join(p.Dir, string(instance)+".tf.json")
	log.Debugln("destroy instance", fp)
	err := p.fs.Remove(fp)
	if err != nil {
		return err
	}
	return p.terraformApply()
}

// DescribeInstances returns descriptions of all instances matching all of the provided tags.
func (p *plugin) DescribeInstances(tags map[string]string) ([]instance.Description, error) {
	log.Debugln("describe-instances", tags)

	show, err := p.terraformShow()
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile("(.*)(instance-[0-9]+)")
	result := []instance.Description{}
	// now we scan for <instance_type.instance-<timestamp> as keys
scan:
	for k, v := range show {
		matches := re.FindStringSubmatch(k)
		if len(matches) == 3 {
			id := matches[2]

			inst := instance.Description{
				Tags:      terraformTags(v, "tags"),
				ID:        instance.ID(id),
				LogicalID: terraformLogicalID(v),
			}

			if len(tags) == 0 {
				result = append(result, inst)
			} else {
				for k, v := range tags {
					if inst.Tags[k] != v {
						continue scan // we implement AND
					}
				}
				result = append(result, inst)
			}
		}
	}
	log.Debugln("describe-instances result=", result)

	return result, nil
}

func terraformTags(v interface{}, key string) map[string]string {
	log.Debugln("terraformTags", v)
	m, ok := v.(map[string]interface{})
	if !ok {
		log.Debugln("terraformTags: return nil")
		return nil
	}
	tags := map[string]string{}
	if mm, ok := m[key].(map[string]interface{}); ok {
		for k, v := range mm {
			tags[k] = fmt.Sprintf("%v", v)
		}
		return tags
	} else if mm, ok := m[key].([]interface{}); ok {
		// add each tag in the list to the tags map
		for _, v := range mm {
			value := fmt.Sprintf("%v", v)
			if strings.Contains(value, ":") {
				log.Debugln("terraformTags system tags detected v=", v)
				vv := strings.Split(value, ":")
				if len(vv) == 2 {
					tags[vv[0]] = vv[1]
				} else {
					log.Errorln("terraformTags: ignore invalid tag detected", value)
				}
			} else {
				tags[value] = "" // for list but no ':"
			}
		}
		log.Debugln("terraformTags return tags", tags)
		return tags
	} else {
		log.Errorln("terraformTags: invalid terraformTags tags value", m[key])
	}

	for k, v := range m {
		if k != "tags.%" && strings.Index(k, "tags.") == 0 {
			n := k[len("tags."):]
			tags[n] = fmt.Sprintf("%v", v)
		}
	}
	return tags
}
func terraformLogicalID(v interface{}) *instance.LogicalID {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	tags, ok := m["tags"].(map[string]interface{})
	if !ok {
		return nil
	}
	v, exists := tags["LogicalID"]
	if exists {
		id := instance.LogicalID(fmt.Sprintf("%v", v))
		return &id
	}
	return nil
}
