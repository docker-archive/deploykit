package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/docker/infrakit/pkg/types"
)

var (
	headerData = []byte("data:")
)

// Options contain client options
type Options struct {
	// SocketDir is the directory to look for socket files for unix:// urls
	SocketDir string
}

func processEvent(msg []byte) []byte {
	switch h := msg; {
	case bytes.Contains(h, headerData):
		return trimHeader(len(headerData), msg)
	default:
		return nil
	}
}

func trimHeader(size int, data []byte) []byte {
	data = data[size:]
	// Remove optional leading whitespace
	if data[0] == 32 {
		data = data[1:]
	}
	// Remove trailing new line
	if data[len(data)-1] == 10 {
		data = data[:len(data)-1]
	}
	return data
}

func socketClient(u *url.URL, socketDir string) (*http.Client, error) {
	socketPath := filepath.Join(socketDir, u.Host)
	if f, err := os.Stat(socketPath); err != nil {
		return nil, err
	} else if f.Mode()&os.ModeSocket == 0 {
		return nil, fmt.Errorf("not-a-socket:%v", socketPath)
	}
	return &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", socketPath)
			},
		},
	}, nil
}

func httpClient(urlString string, opt Options) (*url.URL, *http.Client, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, nil, err
	}
	switch u.Scheme {

	case "http", "https":
		return u, &http.Client{}, nil
	case "unix":
		// unix: will look for a socket that matches the host name at a
		// directory path set by environment variable.
		c, err := socketClient(u, opt.SocketDir)
		if err != nil {
			return nil, nil, err
		}
		u.Scheme = "http"
		return u, c, nil
	}

	return nil, nil, fmt.Errorf("unsupported url:%s", urlString)

}

// Subscribe subscribes to a topic hosted at given url.  It returns a channel of incoming events and errors
func Subscribe(url, topic string, opt Options, headers ...map[string]string) (<-chan *types.Any, <-chan error, error) {

	u, connection, err := httpClient(url, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	// Setup request, specify stream to connect to
	query := req.URL.Query()
	query.Add("topic", topic)
	req.URL.RawQuery = query.Encode()

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	// Add user specified headers
	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}

	streamCh := make(chan *types.Any)
	errCh := make(chan error)

	go func() {

		resp, err := connection.Do(req)
		if err != nil {
			errCh <- err
			close(errCh)
			close(streamCh)
			return
		}

		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)

		for {
			// Read each new line and process the type of event
			line, err := reader.ReadBytes('\n')
			if err != nil {
				close(streamCh)
				return
			}
			if bytes.Contains(line, headerData) {

				if data := trimHeader(len(headerData), line); data != nil {

					streamCh <- types.AnyBytes(data)

				} else {

					select {
					case errCh <- fmt.Errorf("no data: %v", line):
					}

				}
			}
		}
	}()

	return streamCh, errCh, nil
}
