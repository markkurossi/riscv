//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

// Package logger implements debug logging.
package logger

import (
	"fmt"
	"log"
)

// Level defines logging leve.
type Level int

// Logging levels.
const (
	Error Level = iota
	Info
	Verbose
	Debug
	Trace
)

// Logger defines a logging interface.
type Logger interface {
	// Tracef logs a trace message.
	Tracef(format string, args ...interface{})

	// Debugf logs a debugging message.
	Debugf(format string, args ...interface{})

	// Verbosef logs a verbose info message.
	Verbosef(format string, args ...interface{})

	// Infof logs an info message.
	Infof(format string, args ...interface{})

	// Errorf logs an error message.
	Errorf(format string, args ...interface{})
}

var (
	_ Logger = &Log{}
)

// Log implements embeddable debug logger.
type Log struct {
	Level Level
	Name  string
}

func (l *Log) logf(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s: %s: %s", l.Name, level, msg)
}

// Tracef implements Logger.Tracef.
func (l *Log) Tracef(format string, args ...interface{}) {
	if l.Level < Trace {
		return
	}
	l.logf("TRACE", format, args...)
}

// Debugf implements Logger.Debugf.
func (l *Log) Debugf(format string, args ...interface{}) {
	if l.Level < Debug {
		return
	}
	l.logf("DEBUG", format, args...)
}

// Verbosef implements Logger.Verbosef.
func (l *Log) Verbosef(format string, args ...interface{}) {
	if l.Level < Verbose {
		return
	}
	l.logf("VERBOSE", format, args...)
}

// Infof implements Logger.Infof.
func (l *Log) Infof(format string, args ...interface{}) {
	if l.Level < Info {
		return
	}
	l.logf("INFO", format, args...)
}

// Errorf implements Logger.Errorf.
func (l *Log) Errorf(format string, args ...interface{}) {
	l.logf("ERROR", format, args...)
}
