package hello

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
	"sync"
	"time"
)

type Service interface {
	Run()
	Stop()
	Wait()
	GetState() (interface{}, error)
	DiscoverPlugin(Plugin) (*PluginDiscovery, error)
	CallPlugin(PluginCall) (interface{}, error)
}

// Simple object that checks docker to see if it's running on a leader.
type Server struct {
	lock    sync.Mutex
	options Options
	docker  client.APIClient
	config  <-chan []byte
	stop    chan<- struct{}
	done    <-chan struct{}
}

// Options is for hello server options
type Options struct {
	CheckLeaderInterval time.Duration
	DockerEngineAddress string
	DockerTlsOptions    tlsconfig.Options
}

func (o *Options) BindFlags(flags *pflag.FlagSet) {
	flags.DurationVar(&o.CheckLeaderInterval,
		"check_leader_interval",
		o.CheckLeaderInterval,
		"Interval to poll for leader status")
}

func New(options Options, config <-chan []byte, docker client.APIClient) Service {
	return &Server{
		docker:  docker,
		config:  config,
		options: options,
	}
}

// Stop stops the server
func (h *Server) Stop() {
	if h.stop != nil {
		close(h.stop)
	}
}

// Wait blocks until server stops
func (h *Server) Wait() {
	if h.done != nil {
		<-h.done
	}
}

// Run runs the server in a loop
func (h *Server) Run() {
	if h.stop != nil {
		return
	}

	log.Infoln("Server running")

	stop := make(chan struct{})
	h.stop = stop

	tick := time.Tick(h.options.CheckLeaderInterval)

	done := make(chan struct{})
	h.done = done

	defer close(done)

	for {
		select {
		case <-stop:
			log.Infoln("Stopping server")
			return
		case config := <-h.config:
			h.Update(config)
		case <-tick:
			log.Infoln("checking for leadership")
			h.CheckLeader(context.Background())
		}
	}
}

func (h *Server) GetState() (interface{}, error) {

	leader, _ := h.CheckLeader(context.Background())

	return map[string]interface{}{
		"running": h.stop != nil,
		"leader":  leader,
	}, nil
}

func (h *Server) Update(buff []byte) {
	log.Infoln("Received config:", string(buff))
}

func (h *Server) connectDocker() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	cl, err := NewDockerClient(h.options.DockerEngineAddress, &h.options.DockerTlsOptions)
	if err != nil {
		return err
	}
	h.docker = cl
	return nil
}

func (h *Server) CheckLeader(ctx context.Context) (bool, error) {
	info, err := h.docker.Info(ctx)

	log.Debugln("check-leader: info=", info, "err=", err)

	switch {
	case err == nil:
	case err == client.ErrConnectionFailed:
		if err := h.connectDocker(); err != nil {
			return false, err
		} else {
			info, err = h.docker.Info(ctx)
			log.Infoln("after-retry - check-leader: info=", info, "err=", err)
			if err != nil {
				return false, err
			}
		}
	default:
		log.Warningln("Some unknown err. Can't check leader:", err)
		return false, err

	}

	// inspect itself to see if i am the leader
	node, _, err := h.docker.NodeInspectWithRaw(ctx, info.Swarm.NodeID)
	if err != nil {
		return false, err
	}

	if node.ManagerStatus == nil {
		return false, nil
	}
	log.Infoln("leader=", node.ManagerStatus.Leader)

	return node.ManagerStatus.Leader, nil
}
