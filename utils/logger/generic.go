package logger

import "fmt"

type GenericLogger struct{}

func (g GenericLogger) Infof(format string, a ...interface{}) {
	fmt.Printf("INFO: "+format+"\n", a...)
}

func (g GenericLogger) Info(msg string) {
	fmt.Printf("INFO: %s\n", msg)
}

func (g GenericLogger) Debugf(format string, a ...interface{}) {
	fmt.Printf("DEBUG: "+format+"\n", a...)
}

func (g GenericLogger) Debug(msg string) {
	fmt.Printf("DEBUG: %s\n", msg)
}

func (g GenericLogger) Errorf(format string, a ...interface{}) {
	fmt.Errorf("ERROR: "+format+"\n", a...)
}

func (g GenericLogger) Error(msg string) {
	fmt.Errorf("%s", msg)
}

func (g GenericLogger) Warningf(format string, a ...interface{}) {
	fmt.Printf("WARNING: "+format+"\n", a...)
}

func (g GenericLogger) Warning(msg string) {
	fmt.Printf("WARNING: %s\n", msg)
}

func NewGenericLogger() Logger {
	return GenericLogger{}
}
