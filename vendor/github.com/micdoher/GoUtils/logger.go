package utils

import (
	"bytes"
	"io"
	"log"
)

// Log levels inspired in this list:
// https://github.com/NLog/NLog/wiki/Log-levels
var LogLevel = struct {
	TRACE, DEBUG, INFO, WARN, ERROR, FATAL int
}{ 0, 1, 2, 3, 4, 5, }

type Logger struct {
	Level  int
	logger *log.Logger
}

func NewLogger(facility io.Writer, level int) *Logger {
	if facility == nil {
		facility = new(bytes.Buffer)
	}
	return &Logger {
		Level:  level,
		logger: log.New(facility, "", log.Ldate | log.Ltime),
	}
}

func (l *Logger) Trace(msg string, v ...interface{}) {
	l.log(LogLevel.TRACE, msg, v...)
}

func (l *Logger) Debug(msg string, v ...interface{}) {
	l.log(LogLevel.DEBUG, msg, v...)
}

func (l *Logger) Info(msg string, v ...interface{}) {
	l.log(LogLevel.INFO, msg, v...)
}

func (l *Logger) Warn(msg string, v ...interface{}) {
	l.log(LogLevel.WARN, msg, v...)
}

func (l *Logger) Error(msg string, v ...interface{}) {
	l.log(LogLevel.ERROR, msg, v...)
}

func (l *Logger) Fatal(msg string, v ...interface{}) {
	l.log(LogLevel.FATAL, msg, v...)
}

func (l *Logger) Print(msg string, v ...interface{}) {
	l.logger.Printf(msg, v...)
}

func (l *Logger) log(level int, msg string, v ...interface{}) {
	if l.Level <= level {
		l.logger.Printf(msg, v...)
	}
}
