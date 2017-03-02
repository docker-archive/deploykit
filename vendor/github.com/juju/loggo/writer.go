// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package loggo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/juju/ansiterm"
)

// DefaultWriterName is the name of the default writer for
// a Context.
const DefaultWriterName = "default"

// Writer is implemented by any recipient of log messages.
type Writer interface {
	// Write writes a message to the Writer with the given level and module
	// name. The filename and line hold the file name and line number of the
	// code that is generating the log message; the time stamp holds the time
	// the log message was generated, and message holds the log message
	// itself.
	Write(entry Entry)
}

// NewMinLevelWriter returns a Writer that will only pass on the Write calls
// to the provided writer if the log level is at or above the specified
// minimum level.
func NewMinimumLevelWriter(writer Writer, minLevel Level) Writer {
	return &minLevelWriter{
		writer: writer,
		level:  minLevel,
	}
}

type minLevelWriter struct {
	writer Writer
	level  Level
}

// Write writes the log record.
func (w minLevelWriter) Write(entry Entry) {
	if entry.Level < w.level {
		return
	}
	w.writer.Write(entry)
}

type simpleWriter struct {
	writer    io.Writer
	formatter func(entry Entry) string
}

// NewSimpleWriter returns a new writer that writes log messages to the given
// io.Writer formatting the messages with the given formatter.
func NewSimpleWriter(writer io.Writer, formatter func(entry Entry) string) Writer {
	if formatter == nil {
		formatter = DefaultFormatter
	}
	return &simpleWriter{writer, formatter}
}

func (simple *simpleWriter) Write(entry Entry) {
	logLine := simple.formatter(entry)
	fmt.Fprintln(simple.writer, logLine)
}

func defaultWriter() Writer {
	return NewColorWriter(os.Stderr)
}

type colorWriter struct {
	writer *ansiterm.Writer
}

var (
	// SeverityColor defines the colors for the levels output by the ColorWriter.
	SeverityColor = map[Level]*ansiterm.Context{
		TRACE:   ansiterm.Foreground(ansiterm.Default),
		DEBUG:   ansiterm.Foreground(ansiterm.Green),
		INFO:    ansiterm.Foreground(ansiterm.BrightBlue),
		WARNING: ansiterm.Foreground(ansiterm.Yellow),
		ERROR:   ansiterm.Foreground(ansiterm.BrightRed),
		CRITICAL: &ansiterm.Context{
			Foreground: ansiterm.White,
			Background: ansiterm.Red,
		},
	}
	// LocationColor defines the colors for the location output by the ColorWriter.
	LocationColor = ansiterm.Foreground(ansiterm.BrightBlue)
)

// NewColorWriter will write out colored severity levels if the writer is
// outputting to a terminal.
func NewColorWriter(writer io.Writer) Writer {
	return &colorWriter{ansiterm.NewWriter(writer)}
}

// Write implements Writer.
func (w *colorWriter) Write(entry Entry) {
	ts := formatTime(entry.Timestamp)
	// Just get the basename from the filename
	filename := filepath.Base(entry.Filename)

	fmt.Fprintf(w.writer, "%s ", ts)
	SeverityColor[entry.Level].Fprintf(w.writer, entry.Level.Short())
	fmt.Fprintf(w.writer, " %s ", entry.Module)
	LocationColor.Fprintf(w.writer, "%s:%d ", filename, entry.Line)
	fmt.Fprintln(w.writer, entry.Message)
}
