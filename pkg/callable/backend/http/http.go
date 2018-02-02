package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/run/scope"
)

func init() {
	backend.Register("http", HTTP, nil)
}

// HTTP takes a method parameter (string) and a URL (string) and then
// performs the http operation with the rendered data
func HTTP(scope scope.Scope, test bool, opt ...interface{}) (backend.ExecFunc, error) {

	if len(opt) < 2 {
		return nil, fmt.Errorf("requires at least two parameters: first method (string), second url (string)")
	}

	method := "POST"
	url := ""

	method, is := opt[0].(string)
	if !is {
		return nil, fmt.Errorf("method must be string")
	}

	url, is = opt[1].(string)
	if !is {
		return nil, fmt.Errorf("url must be string")
	}

	headers := map[string]string{}
	// remaining are headers
	for i := 2; i < len(opt); i++ {
		h, is := opt[i].(string)
		if !is {
			return nil, fmt.Errorf("header spec must be a string %v", opt[i])
		}
		parts := strings.SplitN(h, "=", 2)
		if len(parts) == 2 {
			headers[parts[0]] = parts[1]
		}
	}

	return func(ctx context.Context, script string, parameters backend.Parameters, args []string) error {

		body := bytes.NewBufferString(script)
		client := &http.Client{}

		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return err
		}

		req.Header.Set("User-Agent", "infrakit-cli/0.5")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("error %s", resp.Status)
		}

		defer resp.Body.Close()
		_, err = io.Copy(backend.GetWriter(ctx), resp.Body)
		return err
	}, nil
}
