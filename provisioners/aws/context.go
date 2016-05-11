package aws

import (
	"github.com/docker/libmachete"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/net/context"
)

// See http://blog.golang.org/context on Context pattern
// and http://blog.golang.org/context/userip/userip.go for derived contexts

type contextKey int

const (
	regionKey contextKey = iota
)

func BuildContextFromKVPair(parent context.Context, m libmachete.KVPair) context.Context {
	t := &struct {
		Region string `json:"region" yaml:"region"`
	}{}
	err := mapstructure.Decode(m, t)
	if err == nil {
		return BuildContext(parent, t.Region)
	}
	return parent
}

// BuildContext returns a context that's properly configured with the required context data.
func BuildContext(parent context.Context, region string) context.Context {
	return WithRegion(parent, region)
}

// WithRegion returns a new context given a parent context and the region.
// For valid value of region, see http://docs.aws.amazon.com/general/latest/gr/rande.html
func WithRegion(parent context.Context, region string) context.Context {
	copy := region
	return context.WithValue(parent, regionKey, &copy)
}

// RegionFromContext returns the Azure region from the request context.
func RegionFromContext(ctx context.Context) (*string, bool) {
	v, ok := ctx.Value(regionKey).(*string)
	return v, ok
}
