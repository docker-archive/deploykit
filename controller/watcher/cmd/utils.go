package main

import (
	"bytes"
	"encoding/base64"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"golang.org/x/net/context"
	"text/template"
)

func getMap(v interface{}, key string) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		if mm, ok := m[key]; ok {
			if r, ok := mm.(map[string]interface{}); ok {
				return r
			}
		}
	}
	return nil
}

func getWorkerJoinToken(docker client.APIClient) (string, error) {
	swarmStatus, err := docker.SwarmInspect(context.Background())
	if err != nil {
		return "", err
	}
	return swarmStatus.JoinTokens.Worker, nil
}

func evaluateFieldsAsTemplate(obj interface{}, ctx interface{}) interface{} {
	configAsMap, ok := obj.(map[string]interface{})
	if !ok {
		log.Infoln("Not a map:", obj)
		// No change
		return obj
	}
	output := map[string]interface{}{}
	for key, value := range configAsMap {
		output[key] = value
		log.Debugln("Processing key", key, "value=", value)
		switch value := value.(type) {
		case string:

			// in case it's base64 encoded... unpack it
			base64Decoded := false
			data, err := base64.StdEncoding.DecodeString(value)
			if err == nil {
				value = string(data)
				log.Infoln("Start value:", value)
				base64Decoded = true
			}

			// treat string fields as though it's a template and evaluate any template variables based
			// on some context
			tpl, err := template.New(key).Parse(value)
			if err != nil {
				log.Warningln("Value", value, "can't be parsed as template. Skipped.")
				continue
			}
			var buff bytes.Buffer
			err = tpl.Execute(&buff, ctx)
			if err != nil {
				log.Warningln("Key", key, "can't be evaluated as template. Err=", err, "Skipped.")
				continue
			}
			if base64Decoded {
				output[key] = base64.StdEncoding.EncodeToString(buff.Bytes())
				log.Infoln("Changed base64 encoded value", output[key], "from=", string(buff.Bytes()))
			} else {
				output[key] = string(buff.Bytes())
			}
		case []interface{}:
			ar := []interface{}{}
			for _, obj := range value {
				ar = append(ar, evaluateFieldsAsTemplate(obj, ctx))
			}
			output[key] = ar
		default:
			output[key] = evaluateFieldsAsTemplate(value, ctx)
		}
	}
	return output
}
