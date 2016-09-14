package mock

//go:generate mockgen -package instance -destination spi/instance/instance.go github.com/docker/libmachete/spi/instance Plugin
//go:generate mockgen -package client -destination docker/engine-api/client/api.go github.com/docker/engine-api/client APIClient
//go:generate mockgen -package scaler -destination plugin/group/group.go github.com/docker/libmachete/plugin/group Scaled
