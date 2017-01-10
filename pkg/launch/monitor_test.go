package launch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Cmd  string
	Args []string
}

type testLauncher struct {
	name     string
	t        *testing.T
	callback func(*Config)
}

func (l *testLauncher) Name() string {
	return l.name
}

func (l *testLauncher) Exec(name string, config *Config) (<-chan error, error) {
	rule := testConfig{}
	err := config.Unmarshal(&rule)
	if err != nil {
		return nil, err
	}
	c := make(chan error)
	l.callback(config)
	close(c)
	return c, nil
}

func TestMonitorLoopNoRules(t *testing.T) {
	monitor := NewMonitor(&testLauncher{
		name: "test",
		t:    t,
	}, []Rule{})

	input, err := monitor.Start()
	require.NoError(t, err)
	require.NotNil(t, input)

	errChan := make(chan error)

	input <- StartPlugin{
		Plugin: "test",
		Error: func(config *Config, e error) {
			errChan <- e
		},
	}

	err = <-errChan
	require.Equal(t, errNoConfig, err)

	monitor.Stop()
}

func TestMonitorLoopValidRule(t *testing.T) {

	raw := &Config{}
	config := &testConfig{
		Cmd:  "hello",
		Args: []string{"world", "hello"},
	}

	rawErr := raw.Marshal(config)
	require.NoError(t, rawErr)
	require.True(t, len([]byte(*raw)) > 0)

	var receivedArgs *Config
	rule := Rule{
		Plugin: "hello",
		Exec:   "test",
		Launch: raw,
	}
	monitor := NewMonitor(&testLauncher{
		name: "test",
		t:    t,
		callback: func(c *Config) {
			receivedArgs = c
		},
	}, []Rule{rule})

	input, err := monitor.Start()
	require.NoError(t, err)
	require.NotNil(t, input)

	started := make(chan interface{})
	input <- StartPlugin{
		Plugin: "hello",
		Started: func(config *Config) {
			close(started)
		},
	}

	<-started

	expected := &Config{}
	err = expected.Marshal(config)
	require.NoError(t, err)

	require.Equal(t, *expected, *receivedArgs)

	monitor.Stop()
}
