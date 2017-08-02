package launch

import (
	"testing"

	"github.com/docker/infrakit/pkg/types"
	"github.com/stretchr/testify/require"
)

type testConfig struct {
	Cmd  string
	Args []string
}

type testLauncher struct {
	name     string
	t        *testing.T
	callback func(*types.Any)
}

func (l *testLauncher) Name() string {
	return l.name
}

func (l *testLauncher) Exec(name string, config *types.Any) (<-chan error, error) {
	rule := testConfig{}
	err := config.Decode(&rule)
	if err != nil {
		return nil, err
	}
	c := make(chan error)
	l.callback(config)
	close(c)
	return c, nil
}

func TestMonitorLoopNoRules(t *testing.T) {
	monitor := NewMonitor([]Exec{
		&testLauncher{
			name: "test",
			t:    t,
		},
	}, []Rule{})

	input, err := monitor.Start()
	require.NoError(t, err)
	require.NotNil(t, input)

	errChan := make(chan error)

	input <- StartPlugin{
		Plugin: "test",
		Exec:   ExecName("test"),
		Error: func(config *types.Any, e error) {
			errChan <- e
		},
	}

	err = <-errChan
	require.Equal(t, errNoConfig, err)

	monitor.Stop()
}

func TestMonitorLoopValidRule(t *testing.T) {

	config := &testConfig{
		Cmd:  "hello",
		Args: []string{"world", "hello"},
	}

	var receivedArgs *types.Any
	rule := Rule{
		Plugin: "hello",
		Launch: map[ExecName]*types.Any{
			"test": types.AnyValueMust(config),
		},
	}
	monitor := NewMonitor([]Exec{
		&testLauncher{
			name: "test",
			t:    t,
			callback: func(c *types.Any) {
				receivedArgs = c
			},
		},
	}, []Rule{rule})

	input, err := monitor.Start()
	require.NoError(t, err)
	require.NotNil(t, input)

	started := make(chan interface{})
	input <- StartPlugin{
		Plugin: "hello",
		Exec:   ExecName("test"),
		Started: func(config *types.Any) {
			close(started)
		},
	}

	<-started

	expected := types.AnyValueMust(config)
	require.Equal(t, *expected, *receivedArgs)

	monitor.Stop()
}

func TestMonitorLoopRuleLookupBehavior(t *testing.T) {

	config := &testConfig{
		Cmd:  "hello",
		Args: []string{"world", "hello"},
	}

	var receivedArgs *types.Any
	rule := Rule{
		Plugin: "hello",
		Launch: map[ExecName]*types.Any{
			"test": types.AnyValueMust(config),
		},
	}
	monitor := NewMonitor([]Exec{
		&testLauncher{
			name: "test",
			t:    t,
			callback: func(c *types.Any) {
				receivedArgs = c
			},
		},
	}, []Rule{rule})

	input, err := monitor.Start()
	require.NoError(t, err)
	require.NotNil(t, input)

	started := make(chan interface{})
	input <- StartPlugin{
		Plugin: "hello",
		Exec:   ExecName("test"),
		Started: func(config *types.Any) {
			close(started)
		},
	}

	<-started

	expected := types.AnyValueMust(config)
	require.Equal(t, *expected, *receivedArgs)

	monitor.Stop()
}

func TestMonitorLoopRuleOverrideOptions(t *testing.T) {

	config := &testConfig{
		Cmd:  "hello",
		Args: []string{"world", "hello"},
	}

	var receivedArgs *types.Any
	rule := Rule{
		Plugin: "hello",
		Launch: map[ExecName]*types.Any{
			"test": types.AnyValueMust(config),
		},
	}
	monitor := NewMonitor([]Exec{
		&testLauncher{
			name: "test",
			t:    t,
			callback: func(c *types.Any) {
				receivedArgs = c
			},
		},
	}, []Rule{rule})

	input, err := monitor.Start()
	require.NoError(t, err)
	require.NotNil(t, input)

	options := map[string]interface{}{
		"some":   "override",
		"values": true,
	}

	started := make(chan interface{})
	input <- StartPlugin{
		Plugin:  "hello",
		Exec:    ExecName("test"),
		Options: types.AnyValueMust(options),
		Started: func(config *types.Any) {
			close(started)
		},
	}

	<-started

	expected := types.AnyValueMust(options)
	require.Equal(t, *expected, *receivedArgs)

	monitor.Stop()
}
