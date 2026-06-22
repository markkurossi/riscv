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

// Logger implements debug logger.
type Logger struct {
	Level Level
	Name  string
}

func (logger *Logger) logf(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s: %s: %s", logger.Name, level, msg)
}

// Tracef logs a trace message.
func (logger *Logger) Tracef(format string, args ...interface{}) {
	if logger.Level < Trace {
		return
	}
	logger.logf("TRACE", format, args...)
}

// Debugf logs a debugging message.
func (logger *Logger) Debugf(format string, args ...interface{}) {
	if logger.Level < Debug {
		return
	}
	logger.logf("DEBUG", format, args...)
}

// Verbosef logs a verbose info message.
func (logger *Logger) Verbosef(format string, args ...interface{}) {
	if logger.Level < Verbose {
		return
	}
	logger.logf("VERBOSE", format, args...)
}

// Infof logs an info message.
func (logger *Logger) Infof(format string, args ...interface{}) {
	if logger.Level < Info {
		return
	}
	logger.logf("INFO", format, args...)
}

// Errorf logs an error message.
func (logger *Logger) Errorf(format string, args ...interface{}) {
	logger.logf("ERROR", format, args...)
}
