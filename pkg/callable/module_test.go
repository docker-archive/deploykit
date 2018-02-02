package callable

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"testing"

	"github.com/docker/infrakit/pkg/callable/backend"
	"github.com/docker/infrakit/pkg/discovery"
	"github.com/docker/infrakit/pkg/discovery/local"
	"github.com/docker/infrakit/pkg/run/scope"
	"github.com/stretchr/testify/require"

	_ "github.com/docker/infrakit/pkg/callable/backend/sh"
)

func dir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}

func testScope(name string) scope.Scope {

	tempDir, err := ioutil.TempDir("", name)
	if err != nil {
		panic(err)
	}

	d, err := local.NewPluginDiscoveryWithDir(tempDir)
	if err != nil {
		panic(err)
	}

	return scope.DefaultScope(func() discovery.Plugins { return d })
}

func linesFunc(t *testing.T, wg *sync.WaitGroup, c *Callable, header string, count int, expect string) {

	defer wg.Done()

	err := c.SetParameter("header", header)
	require.NoError(t, err)

	err = c.SetParameter("lines", count)
	require.NoError(t, err)

	// capture output -- this overrides the defaults set in the Module
	var buff bytes.Buffer

	// another call. this time, we expect the local buffer to be updated
	err = c.Execute(context.Background(), nil, &buff)
	require.NoError(t, err)

	require.Equal(t, expect, buff.String())

}

func TestModule(t *testing.T) {

	var defaultOutput bytes.Buffer

	module := Module{
		Scope:    testScope("testModule"),
		IndexURL: "file://" + path.Join(dir(), "test/index.yml"),
		ParametersFunc: func() backend.Parameters {
			return &Parameters{}
		},
		Options: Options{
			OutputFunc:    func() io.Writer { return &defaultOutput },
			ErrOutputFunc: func() io.Writer { return os.Stderr },
		},
	}

	err := module.Load()
	require.NoError(t, err)

	start, err := module.GetCallable("start")
	require.NoError(t, err)
	require.NotNil(t, start)

	ondemand, err := module.GetCallable("ondemand")
	require.NoError(t, err)
	require.NotNil(t, ondemand)

	spot, err := module.GetCallable("spot")
	require.NoError(t, err)
	require.NotNil(t, spot)

	names := module.List()
	require.Equal(t, []string{"lines", "ondemand", "spot", "start"}, names)

	lines, err := module.GetCallable("lines")
	require.NoError(t, err)
	require.NotNil(t, lines)

	err = lines.Parameters.SetParameter("header", "Test!")
	require.NoError(t, err)

	header, err := lines.Parameters.GetString("header")
	require.NoError(t, err)

	require.Equal(t, "Test!", header)

	err = lines.Parameters.SetParameter("lines", 10)
	require.NoError(t, err)

	err = lines.Execute(context.Background(), nil, nil)
	require.NoError(t, err)

	require.Equal(t, `Test! 1
Test! 2
Test! 3
Test! 4
Test! 5
Test! 6
Test! 7
Test! 8
Test! 9
Test! 10
`, defaultOutput.String())

	err = lines.Parameters.SetParameter("header", "Boom!")
	require.NoError(t, err)

	// capture output -- this overrides the defaults set in the Module
	var buff bytes.Buffer

	// another call. this time, we expect the local buffer to be updated
	err = lines.Execute(context.Background(), nil, &buff)
	require.NoError(t, err)

	require.Equal(t, `Test! 1
Test! 2
Test! 3
Test! 4
Test! 5
Test! 6
Test! 7
Test! 8
Test! 9
Test! 10
`, defaultOutput.String()) // no change

	require.Equal(t, `Boom! 1
Boom! 2
Boom! 3
Boom! 4
Boom! 5
Boom! 6
Boom! 7
Boom! 8
Boom! 9
Boom! 10
`, buff.String()) // results captured locally in new invocation

	// Test for multiple callables for thread safety
	var wg sync.WaitGroup

	{
		t.Log("single thread")

		wg.Add(2)

		linesFunc(t, &wg, lines, "Single", 8, `Single 1
Single 2
Single 3
Single 4
Single 5
Single 6
Single 7
Single 8
`)

		linesFunc(t, &wg, lines, "Thread", 2, `Thread 1
Thread 2
`)
		wg.Wait()
	}

	{
		t.Log("concurrent")

		wg.Add(2)

		lines1, err := lines.Clone(module.ParametersFunc())
		require.NoError(t, err)

		go linesFunc(t, &wg, lines1, "Hello", 10, `Hello 1
Hello 2
Hello 3
Hello 4
Hello 5
Hello 6
Hello 7
Hello 8
Hello 9
Hello 10
`)

		// must get a new callable
		lines2, err := lines.Clone(module.ParametersFunc())
		require.NoError(t, err)

		go linesFunc(t, &wg, lines2, "World", 5, `World 1
World 2
World 3
World 4
World 5
`)

		wg.Wait()
	}
}
