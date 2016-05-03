package console

import (
	"log"
	"os"
)

//go:generate mockgen -package mock -destination mock/mock_console.go github.com/docker/libmachete/cmd/machete/console Console

// Console provides an abstraction for text output, useful for validating output in tests.
type Console interface {
	Println(v ...interface{})

	ErrPrintln(v ...interface{})

	Fatal(v ...interface{})
}

type logConsole struct {
	stderr *log.Logger
	stdout *log.Logger
}

// New creates a console backed by log.Logger.
func New() Console {
	return &logConsole{stderr: log.New(os.Stderr, "", 0), stdout: log.New(os.Stdout, "", 0)}
}

func (l logConsole) Println(v ...interface{}) {
	l.stdout.Println(v)
}

func (l logConsole) ErrPrintln(v ...interface{}) {
	l.stderr.Println(v)
}

func (l logConsole) Fatal(v ...interface{}) {
	l.stderr.Fatal(v)
}
