package fsm

import (
	"flag"
)

const (
	// BufferedChannelSize is the size of the event channel allocated in the set.
	// TODO(chungers) -- implement a FIFO instead of relying on the buffered channel
	BufferedChannelSize = 2000
)

func init() {
	flag.Parse()
}
