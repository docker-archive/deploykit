package mock

//go:generate mockgen -package gcloud -destination gcloud/api.go github.com/docker/infrakit.gcp/plugin/gcloud API
//go:generate mockgen -package gcloud -destination gcloud/apiMetadata.go github.com/docker/infrakit.gcp/plugin/gcloud APIMetadata
//go:generate mockgen -package flavor -destination flavor/flavor.go github.com/docker/infrakit/pkg/spi/flavor Plugin
