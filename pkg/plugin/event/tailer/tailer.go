package tailer

import (
	"fmt"

	logutil "github.com/docker/infrakit/pkg/log"
	"github.com/docker/infrakit/pkg/spi/event"
	"github.com/docker/infrakit/pkg/types"
	"github.com/hpcloud/tail"
)

var (
	log = logutil.New("module", "run/plugin/event/tailer")
)

// Options contains configuration parameters for the tailer
type Options []Rule

// Rule contains configuration parameters for a single file
type Rule struct {
	Path      string
	ReOpen    bool // Reopen recreated files (tail -F)
	MustExist bool // Fail early if the file does not exist
	Pipe      bool // Is a named pipe (mkfifo)

	tailer *tail.Tail
}

// NewPlugin creates a tailer plugin
func NewPlugin(options Options) (*Tailer, error) {
	t := &Tailer{
		options:      options,
		stop:         make(chan struct{}),
		topics:       map[string]interface{}{},
		tailers:      map[string]Rule{},
		currentLines: map[string][]byte{},
	}

	// create the tailer for each rule/ file
	for _, r := range t.options {
		rule := r
		if _, has := t.tailers[rule.Path]; has {
			return nil, fmt.Errorf("duplicate path: %v", rule.Path)
		}

		config := tail.Config{
			Follow:    true,
			Logger:    tail.DiscardingLogger,
			ReOpen:    rule.ReOpen,
			MustExist: rule.MustExist,
			Pipe:      rule.Pipe,
		}

		tailer, err := tail.TailFile(rule.Path, config)
		if err != nil {
			return nil, err
		}
		rule.tailer = tailer
		t.tailers[rule.Path] = rule
		log.Debug("tailer ready", "path", rule.Path, "config", config)
	}

	for file := range t.tailers {
		topic := types.PathFromString(file)
		types.Put(topic, t.getCurrentLine(file), t.topics)
	}

	return t, nil
}

// Tailer is a tailer of multiple files. It is an event source that with topics that match the file paths
type Tailer struct {
	options      Options
	stop         chan struct{}
	topics       map[string]interface{}
	tailers      map[string]Rule
	currentLines map[string][]byte
}

// Stop stops the tailer
func (t *Tailer) Stop() {
	if t.stop != nil {
		close(t.stop)
	}
}

// Data returns the metadata data (current line of each file)
func (t *Tailer) Data() map[string]interface{} {
	return t.topics
}

func (t *Tailer) getCurrentLine(file string) func() interface{} {
	return func() interface{} {
		return string(t.currentLines[file])
	}
}

// List returns the nodes under the given topic
func (t *Tailer) List(topic types.Path) ([]string, error) {
	return types.List(topic, t.topics), nil
}

const (
	tailerType = event.Type("tailer")
)

// PublishOn sets the channel to publish on
func (t *Tailer) PublishOn(c chan<- *event.Event) {

	latest := make(chan event.Event, 100)

	go func() {
		defer func() {
			close(c)
			close(latest)
		}()

		log.Info("Time starting publish", "channel", c)
		for _, rule := range t.tailers {
			go func(r Rule) {
				for line := range r.tailer.Lines {

					c <- event.Event{
						Timestamp: line.Time,
						Type:      tailerType,
						ID:        r.Path,
					}.Init().WithTopic(r.Path).WithDataMust(line.Text)
					log.Debug("tail", "file", r.Path, "line", line)
				}
			}(rule)
		}

		for {
			select {
			case <-t.stop:
				for _, r := range t.tailers {
					r.tailer.Stop()
				}
				return
			case v := <-latest:
				t.currentLines[v.ID] = v.Data.Bytes()
			}
		}

	}()
}
