package client

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/types"
)

var (
	headerData = []byte("data:")

	log = logutil.New("module", "broker/client")
)

// Options contain client options
type Options struct {
	// SocketDir is the directory to look for socket files for unix:// urls
	SocketDir string

	// Path is the URL path, if the server's handler is at the mux path (e.g. /events)
	Path string
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

func socketClient(u *url.URL, socketDir string) (*http.Client, *http.Transport, error) {
	socketPath := path.Join(socketDir, u.Host)
	if f, err := os.Stat(socketPath); err != nil {
		return nil, nil, err
	} else if f.Mode()&os.ModeSocket == 0 {
		return nil, nil, fmt.Errorf("not-a-socket:%v", socketPath)
	}

	tsport := http.Transport{
		Dial: func(proto, addr string) (conn net.Conn, err error) {
			return net.Dial("unix", socketPath)
		},
	}
	return &http.Client{
		Transport: &tsport,
	}, &tsport, nil
}

func httpClient(urlString string, opt Options) (*url.URL, *http.Client, *http.Transport, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, nil, nil, err
	}
	switch u.Scheme {

	case "http", "https":
		tsport := http.DefaultTransport
		return u, &http.Client{
			Transport: tsport,
		}, tsport.(*http.Transport), nil
	case "unix":
		// unix: will look for a socket that matches the host name at a
		// directory path set by environment variable.
		c, tsport, err := socketClient(u, opt.SocketDir)
		if err != nil {
			return nil, nil, tsport, err
		}
		u.Scheme = "http"
		u.Host = "e"
		u.Path = "/"
		return u, c, tsport, nil
	}

	return nil, nil, nil, fmt.Errorf("unsupported url:%s", urlString)

}

// Subscribe subscribes to a topic hosted at given url.  It returns a channel of incoming events and errors
// as well as done which will close the connection and exit the subscriber.
func Subscribe(url, topic string, opt Options) (
	messages <-chan *types.Any,
	errors <-chan error,
	done chan<- struct{},
	err error,
) {

	u, connection, tsport, err := httpClient(url, opt)
	if err != nil {
		return nil, nil, nil, err
	}

	if opt.Path != "" {
		u.Path = path.Join(u.Path, opt.Path)
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, nil, nil, err
	}

	// Setup request, specify stream to connect to
	query := req.URL.Query()
	if query["topic"] == nil {
		query.Add("topic", topic)
		req.URL.RawQuery = query.Encode()
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	streamCh := make(chan *types.Any)
	doneCh := make(chan struct{})
	errCh := make(chan error)

	go func() {

		resp, err := connection.Do(req)
		if err != nil {
			errCh <- err
			close(errCh)
			close(streamCh)
			return
		}

		defer func() {
			resp.Body.Close()
			log.Debug("canceling request", "req", req)
			tsport.CancelRequest(req)
			close(streamCh)
			close(errCh)
		}()

		if resp.StatusCode != http.StatusOK {
			errCh <- fmt.Errorf("http-status:%v", resp.StatusCode)
			return
		}

		reader := bufio.NewReader(resp.Body)

		for {
			select {

			case <-doneCh:
				log.Info("close", "connection", connection)
				return

			default:
			}

			// Read each new line and process the type of event
			line, err := reader.ReadBytes('\n')

			if err != nil {
				errCh <- err
				return
			}
			if bytes.Contains(line, headerData) {

				if data := trimHeader(len(headerData), line); len(data) > 0 {

					streamCh <- types.AnyBytes(data)

				} else {

					select {
					case errCh <- fmt.Errorf("no data: %s", string(line)):
					default:
					}

				}
			}
		}
	}()

	return streamCh, errCh, doneCh, nil
}
