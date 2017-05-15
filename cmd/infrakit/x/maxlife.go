package x

import (
	"os"
	"strings"
	"time"

	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/plugin"
	instance_rpc "github.com/docker/infrakit/pkg/rpc/instance"
	"github.com/docker/infrakit/pkg/spi/instance"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

func maxlifeCommand(plugins func() discovery.Plugins) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "maxlife <instance plugin name>...",
		Short: "Sets max life on the given instances",
	}

	//name := cmd.Flags().String("name", "", "Name to use as name of this plugin")
	poll := cmd.Flags().DurationP("poll", "i", 10*time.Second, "Polling interval")
	maxlife := cmd.Flags().DurationP("maxlife", "m", 10*time.Minute, "Max lifetime of the resource")
	flagTags := cmd.Flags().StringSliceP("tag", "t", []string{}, "Tags to filter instance by")

	cmd.RunE = func(c *cobra.Command, args []string) error {

		if len(args) == 0 {
			cmd.Usage()
			os.Exit(-1)
		}

		tags := toTags(*flagTags)

		// Now we have a list of instance plugins to maxlife
		plugins, err := getInstancePlugins(plugins, args)
		if err != nil {
			return err
		}

		// For each we start a goroutine to poll and kill instances
		stops := []chan struct{}{}

		retry := false

	loop:
		for {
			for name, plugin := range plugins {

				stop := make(chan struct{})
				stops = append(stops, stop)

				described, err := plugin.DescribeInstances(tags, false)
				if err != nil {
					log.Warn("cannot get initial count", "name", name, "err", err)
					retry = true
				}

				go ensureMaxlife(name, plugin, stop, *poll, *maxlife, tags, len(described))
			}

			if !retry {
				break loop
			}

			// Wait a little bit before trying again -- use the same poll interval
			<-time.After(*poll)
		}

		return nil
	}

	return cmd
}

func age(instance instance.Description, now time.Time) (age time.Duration) {
	link := types.NewLinkFromMap(instance.Tags)
	if link.Valid() {
		age = now.Sub(link.Created())
	}
	return
}

func maxAge(instances []instance.Description, now time.Time) instance.Description {
	// check to see if the tags of the instances have links.  Links have a creation date and
	// we can use it to compute the age
	var max time.Duration
	var found = 0
	for i, instance := range instances {
		age := age(instance, now)
		if age > max {
			max = age
			found = i
		}
	}
	return instances[found]
}

func ensureMaxlife(name string, plugin instance.Plugin, stop chan struct{}, poll, maxlife time.Duration,
	tags map[string]string, initialCount int) {

	// Count is used to track the steady state...  we don't want to keep killing instances
	// if the counts are steadily decreasing.  The idea here is that once we terminate a resource
	// another one will be resurrected so we will be back to steady state.
	// Of course it's possible that the size of the cluster actually is decreased.  So we'd
	// wait for a few samples to get to steady state before we terminate another instance.
	// Currently we assume damping == 1 or 1 successive samples of delta >= 0 is sufficient to terminate
	// another instance.

	last := initialCount
	tick := time.Tick(poll)
loop:
	for {

		select {

		case now := <-tick:

			described, err := plugin.DescribeInstances(tags, false)
			if err != nil {
				// Transient error?
				log.Warn("error describing instances", "name", name, "err", err)
				continue
			}

			// TODO -- we should compute the 2nd derivative wrt time to make sure we
			// are truly in a steady state...

			current := len(described)
			delta := current - last
			last = current

			if current < 2 {
				log.Info("there are less than 2 instances.  No actions.", "name", name)
				continue
			}

			if delta < 0 {
				// Don't do anything if there are fewer instances at this iteration
				// than the last.  We want to wait until steady state
				log.Info("fewer instances in this round.  No actions taken", "name", name)
				continue
			}

			// Just pick a single oldest instance per polling cycle.  This is so
			// that we don't end up destroying the cluster by taking down too many instances
			// all at once.
			oldest := maxAge(described, now)

			// check to make sure the age is over the maxlife
			if age(oldest, now) > maxlife {
				// terminate it and hope the group controller restores with a new intance
				err = plugin.Destroy(oldest.ID)
				if err != nil {
					log.Warn("cannot destroy instance", "name", name, "id", oldest.ID, "err", err)
					continue
				}
			}

		case <-stop:
			log.Info("stop requested", "name", name)
			break loop
		}
	}

	log.Info("maxlife stopped", "name", name)
	return
}

func toTags(slice []string) map[string]string {
	tags := map[string]string{}

	for _, tag := range slice {
		kv := strings.SplitN(tag, "=", 2)
		if len(kv) != 2 {
			log.Warn("bad format tag", "input", tag)
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key != "" && val != "" {
			tags[key] = val
		}
	}
	return tags
}

func getInstancePlugins(plugins func() discovery.Plugins, names []string) (map[string]instance.Plugin, error) {
	targets := map[string]instance.Plugin{}
	for _, target := range names {
		endpoint, err := plugins().Find(plugin.Name(target))
		if err != nil {
			return nil, err
		}
		if p, err := instance_rpc.NewClient(plugin.Name(target), endpoint.Address); err == nil {
			targets[target] = p
		} else {
			return nil, err
		}
	}
	return targets, nil
}
