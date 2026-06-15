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

type Level int

const (
	Error Level = iota
	Info
	Debug
)

type Logger struct {
	Level Level
	Name  string
}

func (logger *Logger) logf(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s: %s: %s", logger.Name, level, msg)
}

func (logger *Logger) Debugf(format string, args ...interface{}) {
	if logger.Level < Debug {
		return
	}
	logger.logf("DEBUG", format, args...)
}

func (logger *Logger) Infof(format string, args ...interface{}) {
	if logger.Level < Info {
		return
	}
	logger.logf("INFO", format, args...)
}

func (logger *Logger) Errorf(format string, args ...interface{}) {
	logger.logf("ERROR", format, args...)
}
