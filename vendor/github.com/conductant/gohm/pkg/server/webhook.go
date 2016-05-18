package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/golang/glog"
	"net/http"
	"net/url"
	"text/template"
)

var (
	ErrNoServiceDefined = errors.New("no-service-defined")
	ErrNoWebhookDefined = errors.New("no-webhook-defined")

	WebhookHmacHeader = "X-Gohm-Hmac"
)

// Webhook callbacks
type EventKeyUrlMap map[string]Webhook
type Webhook struct {
	Url       string `json:"destination_url"`
	AuthToken string `json:"auth_token,omitempty"`
}
type WebhookMap map[string]EventKeyUrlMap

type WebhookManager interface {
	Send(service string, event string, message interface{}, templateString string) error
	RegisterWebhooks(service string, ekum EventKeyUrlMap) error
	RemoveWebhooks(service string) error
}

func (this *WebhookMap) ToJSON() []byte {
	if buff, err := json.Marshal(this); err == nil {
		return buff
	}
	return nil
}

func (this *WebhookMap) FromJSON(s []byte) error {
	dec := json.NewDecoder(bytes.NewBuffer(s))
	return dec.Decode(this)
}

func (this *EventKeyUrlMap) ToJSON() []byte {
	if buff, err := json.Marshal(this); err == nil {
		return buff
	}
	return nil
}

func (this *EventKeyUrlMap) FromJSON(s []byte) error {
	dec := json.NewDecoder(bytes.NewBuffer(s))
	return dec.Decode(this)
}

// Default in-memory implementation
func (this WebhookMap) Send(serviceKey, eventKey string, message interface{}, templateString string) error {
	m := this[serviceKey]
	if m == nil {
		return ErrNoServiceDefined
	}
	hook, has := m[eventKey]
	if !has {
		return ErrNoWebhookDefined
	}
	return hook.Send(message, templateString)
}

func (this WebhookMap) RegisterWebhooks(serviceKey string, ekum EventKeyUrlMap) error {
	this[serviceKey] = ekum
	return nil
}

func (this WebhookMap) RemoveWebhooks(serviceKey string) error {
	delete(this, serviceKey)
	return nil
}

func (hook *Webhook) Send(message interface{}, templateString string) error {
	url, err := url.Parse(hook.Url)
	if err != nil {
		return err
	}

	go func() {
		glog.Infoln("Sending callback to", url)

		var buffer bytes.Buffer
		if templateString != "" {
			t := template.Must(template.New(templateString).Parse(templateString))
			err := t.Execute(&buffer, message)
			if err != nil {
				glog.Warningln("Cannot build payload for event", message)
				return
			}
		} else {
			glog.Infoln("no-payload", url)
		}
		// Determine where to send the event.
		client := &http.Client{}
		post, err := http.NewRequest("POST", url.String(), &buffer)
		post.Header.Add(WebhookHmacHeader, "TO DO: compute a HMAC here")
		if hook.AuthToken != "" {
			post.Header.Add("Authorization", "Bearer "+hook.AuthToken)
		}
		resp, err := client.Do(post)
		if err != nil {
			glog.Warningln("Cannot deliver callback to", url, "error:", err)
		} else {
			glog.Infoln("Sent callback to ", url, "response=", resp)
		}
	}()
	return nil
}
