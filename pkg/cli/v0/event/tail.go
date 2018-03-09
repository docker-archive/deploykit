package event

import (
	"fmt"
	"path"
	"strings"

	"github.com/docker/infrakit/pkg/cli"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/template"
	"github.com/docker/infrakit/pkg/types"
	"github.com/spf13/cobra"
)

// Tail returns the Tail command
func Tail(name string, services *cli.Services) *cobra.Command {

	tail := &cobra.Command{
		Use:   "tail",
		Short: "Get event entry by path",
	}
	globals := []string{}
	templateURL := "str://{{jsonDecode .Data}}"
	tail.Flags().StringVar(&templateURL, "view", templateURL, "URL for view template")
	tail.RunE = func(cmd *cobra.Command, args []string) error {

		eventPlugin, err := LoadPlugin(services.Scope.Plugins(), name)
		if err != nil {
			return nil
		}
		cli.MustNotNil(eventPlugin, "event plugin not found", "name", name)

		log.Debug("rendering view", "template=", templateURL)
		engine, err := services.Scope.TemplateEngine(templateURL, template.Options{})
		if err != nil {
			return err
		}
		for _, global := range globals {
			kv := strings.Split(global, "=")
			if len(kv) != 2 {
				continue
			}
			key := strings.Trim(kv[0], " \t\n")
			val := strings.Trim(kv[1], " \t\n")
			if key != "" && val != "" {
				engine.Global(key, val)
			}
		}

		if len(args) == 0 {
			args = []string{"."}
		}

		topics := []types.Path{}
		for _, a := range args {
			p := types.PathFromString(a).Clean()
			if p.Valid() {
				topics = append(topics, p)
			}
		}

		if len(topics) == 0 {
			return nil
		}

		collector := make(chan *event.Event)
		done := make(chan int)
		running := 0

		// now subscribe to each topic
		for _, topic := range topics {

			target := *topic.Index(0)

			eventTopic := topic
			if path.Dir(name) != "." {
				eventTopic = types.PathFromString(path.Base(name)).Join(topic)
			}

			client, is := eventPlugin.(event.Subscriber)
			if !is {
				return fmt.Errorf("not a subscriber: %s, %v", target, eventPlugin)
			}

			log.Debug("Subscribing", "topic", eventTopic)

			stream, stop, err := client.SubscribeOn(eventTopic)
			if err != nil {
				return fmt.Errorf("cannot subscribe: %s, err=%v", topic, err)
			}

			defer close(stop)

			go func() {
				defer func() { done <- -1 }()

				for {
					select {
					case evt, ok := <-stream:
						if !ok {
							log.Info("Server disconnected", "topic", topic)
							return
						}

						// Scope the topic
						evt.Topic = topic
						collector <- evt
					}
				}
			}()

			running++
		}

	loop:
		for {
			select {
			case v := <-done:
				running += v
				if running == 0 {
					break loop
				}

			case evt, ok := <-collector:
				if !ok {
					log.Info("Server disconnected.")
					break loop
				}

				// view of the Event
				type ext struct {
					*event.Event
					Properties interface{} // So that golang template can operate via . notation
				}

				view := &ext{Event: evt}
				if err := evt.Data.Decode(&view.Properties); err != nil {
					if view.Error != nil {
						view.Error = errors([]error{view.Error, err})
					} else {
						view.Error = err
					}
				}

				buff, err := engine.Render(view)
				if err != nil {
					log.Warn("error rendering view", "err=", err)
				} else {
					fmt.Println(buff)
				}
			}
		}

		return nil
	}
	return tail
}

type errors []error

func (errs errors) Error() string {
	l := []string{}
	for _, e := range errs {
		l = append(l, e.Error())
	}
	return strings.Join(l, ";")
}
