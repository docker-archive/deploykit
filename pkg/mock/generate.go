package mock

//go:generate mockgen -package instance -destination spi/instance/instance.go github.com/docker/infrakit/pkg/spi/instance Plugin
//go:generate mockgen -package flavor -destination spi/flavor/flavor.go github.com/docker/infrakit/pkg/spi/flavor Plugin
//go:generate mockgen -package group -destination spi/group/group.go github.com/docker/infrakit/pkg/spi/group Plugin
//go:generate mockgen -package client -destination docker/docker/client/api.go github.com/docker/infrakit/pkg/util/docker APIClientCloser
//go:generate mockgen -package group -destination plugin/group/group.go github.com/docker/infrakit/pkg/plugin/group Scaled
//go:generate mockgen -package store -destination store/store.go github.com/docker/infrakit/pkg/store Snapshot
