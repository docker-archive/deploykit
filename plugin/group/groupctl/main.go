package main

import (
	"errors"
	"github.com/docker/libmachete/plugin/group"
	"github.com/docker/libmachete/plugin/group/groupserver"
	"github.com/docker/libmachete/spi/instance"
)

func main() {
	pluginLookup := func(key string) (instance.Plugin, error) {
		switch key {
		case "test":
			return group.NewTestInstancePlugin(), nil
		default:
			return nil, errors.New("Unknown instance plugin")
		}
	}

	groupserver.Run(8888, pluginLookup)
}
