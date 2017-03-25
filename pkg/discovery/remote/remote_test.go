package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

type handler func(http.ResponseWriter, *http.Request)

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(w, r)
}

func TestRemoteDiscovery(t *testing.T) {

	server1 := httptest.NewServer(handler(
		func(w http.ResponseWriter, r *http.Request) {

			require.Equal(t, http.MethodOptions, r.Method)
			resp := DiscoveryResponse{
				Plugins: []string{
					"group",
					"instance-aws",
					"event-timer",
					"flavor-swarm",
				},
			}
			w.Write(types.AnyValueMust(resp).Bytes())
			return
		}))

	// This case there's only one which may not be able to determine the leader status.
	// The proxy will just assume it's a leader. This is mostly for development.
	remotes := ParseURLMust(server1.URL)
	dir, err := NewPluginDiscovery(remotes)
	require.NoError(t, err)

	mDevel, err := dir.List()
	require.NoError(t, err)
	require.Equal(t, 4, len(mDevel)) // 4 plugins as shown above

	for n, endpoint := range mDevel {
		require.Equal(t, endpoint.Address, server1.URL+"/"+n+"/")
	}

	// Now we start up the peers

	server2 := httptest.NewServer(handler(
		func(w http.ResponseWriter, r *http.Request) {

			require.Equal(t, http.MethodOptions, r.Method)
			resp := DiscoveryResponse{
				Leader: true,
				Plugins: []string{
					"group",
					"instance-aws",
					"event-timer",
					"flavor-swarm",
				},
			}
			w.Write(types.AnyValueMust(resp).Bytes())
			return
		}))

	server3 := httptest.NewServer(handler(
		func(w http.ResponseWriter, r *http.Request) {

			require.Equal(t, http.MethodOptions, r.Method)
			resp := DiscoveryResponse{
				Plugins: []string{
					"group",
					"instance-aws",
					"event-timer",
					"flavor-swarm",
				},
			}
			w.Write(types.AnyValueMust(resp).Bytes())
			return
		}))

	// Here we intentionally point to the entire set
	remotes = ParseURLMust(server1.URL, server2.URL, server3.URL)
	dir, err = NewPluginDiscovery(remotes)
	require.NoError(t, err)

	mAll, err := dir.List()
	require.NoError(t, err)

	// now the list should not be the same as when we had a single remote
	require.NotEqual(t, mAll, mDevel)

	for n, endpoint := range mAll {
		require.Equal(t, endpoint.Address, server2.URL+"/"+n+"/")
	}

	// close two servers
	server1.Close()
	server3.Close()

	mSingle, err := dir.List()
	require.NoError(t, err)

	require.Equal(t, mSingle, mAll)

	// find something
	endpoint, err := dir.Find("event-timer")
	require.NoError(t, err)
	require.Equal(t, server2.URL+"/event-timer/", endpoint.Address)

	_, err = dir.Find("not-found")
	require.True(t, discovery.IsErrNotFound(err))

	server2.Close()
}
