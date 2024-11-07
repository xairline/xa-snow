//go:build !test

package main

import (
    "fmt"
    "time"
    "github.com/xairline/xa-snow/services"
)

// MyLogger is a mock type for the Logger type
type MyLogger struct {
	a int
}

func (m *MyLogger) Info(msg string) {
	fmt.Println("Info:", msg)
}

func (m *MyLogger) Debugf(format string, a ...interface{}) {
    fmt.Println("Debug:", fmt.Sprintf(format, a...))
}

func (m *MyLogger) Debug(msg string) {
	fmt.Println(msg)
}

func (m *MyLogger) Error(msg string) {
	fmt.Println(msg)
}

func (m *MyLogger) Warningf(format string, a ...interface{}) {
    fmt.Println("Warning:", fmt.Sprintf(format, a...))
}

func (m *MyLogger) Warning(msg string) {
	fmt.Println("Warning:", msg)
}

func (m *MyLogger) Infof(format string, a ...interface{}) {
    fmt.Println("Info:", fmt.Sprintf(format, a...))
}

func (m *MyLogger) Errorf(format string, a ...interface{}) {
    fmt.Println("Error:", fmt.Sprintf(format, a...))
}

func main() {
	Logger := new(MyLogger)
    Logger.Info("startup")
	gs := services.NewGribService(Logger, ".", "bin")
	_ = gs.DownloadAndProcessGribFile()

    for ! gs.IsReady() {
        Logger.Info("waiting for ready")
        time.Sleep(1)
    }

	s := gs.GetSnowDepth(51.418441, 9.387076)
    Logger.Infof("s = %0.2f", services.SnowDepthToXplaneSnowNow(s))
	s = gs.GetSnowDepth(51.46, 9.387076)
    Logger.Infof("s = %0.2f", services.SnowDepthToXplaneSnowNow(s))
	s = gs.GetSnowDepth(51.418441, 9.32)
    Logger.Infof("s = %0.2f", services.SnowDepthToXplaneSnowNow(s))
	s = gs.GetSnowDepth(51.418441, 9.42)    // to the east
    Logger.Infof("s = %0.2f", services.SnowDepthToXplaneSnowNow(s))
}
