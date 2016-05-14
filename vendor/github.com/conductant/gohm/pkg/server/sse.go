package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"net/http"
	"sync"
)

// TODO - return and disconnect client
func (this *engine) DirectHttpStream(w http.ResponseWriter, r *http.Request) (chan<- interface{}, error) {

	// Make sure that the writer supports flushing.
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming-unsupported")
	}

	// Create a new channel, over which the broker can
	// send this client messages.
	messageChan := make(chan interface{})

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	go func() {
		for {
			// Read from our messageChan.
			msg, open := <-messageChan

			if !open || msg == nil {
				// If our messageChan was closed, this means that the client has
				// disconnected.
				glog.V(100).Infoln("Messages stopped.. Closing http connection")
				break
			}

			// by type switch
			switch t := msg.(type) {
			case string:
				fmt.Fprintf(w, "%s\n", msg.(string))
			case []byte:
				fmt.Fprintf(w, "%s\n", string(msg.([]byte)))
			default:
				fmt.Fprintf(w, "event: %s\n", t)
				fmt.Fprint(w, "data: ")
				json.NewEncoder(w).Encode(&msg)
				fmt.Fprint(w, "\n\n")
			}

			// Flush the response.  This is only possible if
			// the repsonse supports streaming.
			f.Flush()
		}
		// Done.
		glog.V(100).Infoln("Finished HTTP request at ", r.URL.Path)
	}()

	return messageChan, nil
}

func (this *engine) MergeHttpStream(resp http.ResponseWriter, req *http.Request,
	contentType, eventType, key string, source <-chan interface{}) error {

	sc, new := this.StreamChannel(contentType, eventType, key)
	if new {
		go func() {
			// connect the source
			for {
				if m, open := <-source; open {
					sc.messages <- m
				} else {
					glog.Infoln("Source", source, "closed.")
					return
				}
			}
		}()
	}
	sc.ServeHTTP(resp, req)
	return nil
}

func (this *engine) Stop() {
	for _, s := range this.sseChannels {
		s.stop <- 1
	}
}

func (this *engine) deleteSseChannel(key string) {
	this.lock.Lock()
	defer this.lock.Unlock()
	delete(this.sseChannels, key)
	glog.Infoln("Removed sse channel", key, "count=", len(this.sseChannels))
}

func (this *engine) StreamChannel(contentType, eventType, key string) (*sseChannel, bool) {
	this.lock.Lock()
	defer this.lock.Unlock()

	if c, has := this.sseChannels[key]; has {
		return c, false
	} else {
		c = &sseChannel{
			Key:         key,
			ContentType: contentType,
			EventType:   eventType,
			engine:      this,
		}
		c.Init().Start()
		this.sseChannels[key] = c
		return c, true
	}
}

type event_client chan interface{}

type sseChannel struct {
	Key string

	ContentType string
	EventType   string

	engine *engine
	lock   sync.Mutex

	// Send to this to stop
	stop chan int

	clients map[event_client]int

	// Multiple sources can multiplex onto the Messages.  This gives a way to track
	sources        int
	sourceTracking chan int

	// Channel into which new clients can be pushed
	newClients chan event_client

	// Channel into which disconnected clients should be pushed
	defunctClients chan event_client

	// Channel into which messages are pushed to be broadcast out
	// to attahed clients.
	messages chan interface{}
}

func (this *sseChannel) Init() *sseChannel {
	this.stop = make(chan int)
	this.clients = make(map[event_client]int)
	this.newClients = make(chan event_client)
	this.defunctClients = make(chan event_client)
	this.messages = make(chan interface{})
	this.sourceTracking = make(chan int)
	return this
}

func (this *sseChannel) Stop() {
	glog.V(100).Infoln("Stopping channel", this.Key)

	if this.stop == nil {
		glog.V(100).Infoln("Stopped.")
		return
	}

	this.lock.Lock()
	defer this.lock.Unlock()

	glog.V(100).Infoln("Closing stop", this.Key)
	close(this.stop)
	this.stop = nil

	glog.V(100).Infoln("Closing messages", this.Key)
	close(this.messages)
	// stop all clients
	for c, _ := range this.clients {
		glog.V(100).Infoln("Closing event client", c)
		close(c)
	}
	this.engine.deleteSseChannel(this.Key)
}

func (this *sseChannel) Start() *sseChannel {
	go func() {
		defer glog.Infoln("Channel", this.Key, "Stopped.")
		for {
			select {

			case c := <-this.sourceTracking:
				this.sources += c
				if this.sources == 0 {
					// all sources are gone.
					glog.V(100).Infoln("All sources gone. Closing channel:", this.Key)
					this.Stop()
					this.engine.deleteSseChannel(this.Key)
					return
				}
			case s := <-this.newClients:
				this.lock.Lock()
				this.clients[s] = 1
				this.lock.Unlock()
				glog.V(100).Infoln("Added new client:", s)

			case s := <-this.defunctClients:
				this.lock.Lock()
				delete(this.clients, s)
				this.lock.Unlock()
				close(s)
				glog.V(100).Infoln("Removed client:", s)

			case _, open := <-this.stop:
				if open {
					glog.V(100).Infoln("Received stop.", this.Key)
					this.Stop()
				} else {
					glog.V(100).Infoln("Stopping channel loop.", this.Key)
					return
				}
			default:
				msg, open := <-this.messages
				if !open || msg == nil {
					for s, _ := range this.clients {
						this.defunctClients <- s
					}
					this.stop <- 1

					glog.V(100).Infoln("Channel loop stopped", this.Key)
					return // stop this
				} else {
					// There is a new message to send.  For each
					// attached client, push the new message
					// into the client's message channel.
					for s, _ := range this.clients {
						s <- msg
					}
				}
			}
		}
	}()
	return this
}

// TODO - return and disconnect client
func (this *sseChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// Make sure that the writer supports flushing.
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Create a new channel, over which the broker can
	// send this client messages.
	messageChan := make(event_client)

	// Add this client to the map of those that should
	// receive updates
	this.newClients <- messageChan

	// Listen to the closing of the http connection via the CloseNotifier
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// Remove this client from the map of attached clients
		// when `EventHandler` exits.
		this.defunctClients <- messageChan
		glog.V(100).Infoln("HTTP connection just closed.")
	}()

	// Set the headers related to event streaming.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {

		// Read from our messageChan.
		msg, open := <-messageChan

		if !open || msg == nil {
			// If our messageChan was closed, this means that the client has
			// disconnected.
			glog.V(100).Infoln("Messages stopped.. Closing http connection")
			break
		}

		switch this.ContentType {
		case "text/plain":
			fmt.Fprintf(w, "%s\n", msg)
		default:
			fmt.Fprintf(w, "event: %s\n", this.EventType)
			fmt.Fprint(w, "data: ")
			json.NewEncoder(w).Encode(&msg)
			fmt.Fprint(w, "\n\n")
		}

		// Flush the response.  This is only possible if
		// the repsonse supports streaming.
		f.Flush()
	}

	// Done.
	glog.V(100).Infoln("Finished HTTP request at ", r.URL.Path, "num_channels=", len(this.engine.sseChannels))
}
