package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"

	"github.com/docker/infrakit/pkg/types"
)

var (
	headerData = []byte("data:")
)

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

func Subscribe(url, topic string, headers map[string]string) (<-chan interface{}, <-chan error, error) {

	req, err := http.NewRequest("GET", url, nil)
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	streamCh := make(chan interface{})
	errCh := make(chan error)

	go func() {
		connection := &http.Client{}

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
					var message interface{}
					if err := types.AnyBytes(data).Decode(&message); err == nil {
						streamCh <- message
					} else {
						select {
						case errCh <- err:
						}
					}
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
