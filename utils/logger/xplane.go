//go:build !test

package logger

import (
	"github.com/xairline/goplane/extra/logging"
)

type XplaneLogger struct {
}

func (x XplaneLogger) Infof(format string, a ...interface{}) {
	logging.Infof(format, a...)
}

func (x XplaneLogger) Info(msg string) {
	logging.Info(msg)
}

func (x XplaneLogger) Debugf(format string, a ...interface{}) {
	logging.Debugf(format, a...)
}

func (x XplaneLogger) Debug(msg string) {
	logging.Debug(msg)
}

func (x XplaneLogger) Errorf(format string, a ...interface{}) {
	logging.Errorf(format, a...)
}

func (x XplaneLogger) Error(msg string) {
	logging.Error(msg)
}

func (x XplaneLogger) Warningf(format string, a ...interface{}) {
	logging.Warningf(format, a...)
}

func (x XplaneLogger) Warning(msg string) {
	logging.Warning(msg)
}

func NewXplaneLogger() Logger {
	return XplaneLogger{}
}
