//go:build ignore

package main

import (
	"fmt"
	"github.com/xairline/xa-snow/services"
	"time"
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
	logger := new(MyLogger)
	logger.Info("startup")
	gs := services.NewGribService(logger, ".", "bin", services.NewCoastService(logger, "."))
	//_, _ = gs.DownloadAndProcessGribFile(true, 0, 0, 0)
	_, m, _ := gs.DownloadAndProcessGribFile(false, 01, 03, 18)

	p2s := services.NewPhys2XPlane(logger)

	for !gs.IsReady() {
		logger.Info("waiting for ready")
		time.Sleep(1)
	}

	v := m.GetIdx(1820, 1253)
	logger.Infof("m.GetIdx: %f", v)
	s := gs.GetSnowDepth(51.418441, 9.387076)
	sd, saw, icen := p2s.SnowDepthToXplaneSnowNow(s)
	logger.Infof("s = %0.2f, saw = %0.2f, icen = %0.2f", sd, saw, icen)

	s = gs.GetSnowDepth(51.48, 9.387076)
	sd, saw, icen = p2s.SnowDepthToXplaneSnowNow(s)
	logger.Infof("s = %0.2f, saw = %0.2f, icen = %0.2f", sd, saw, icen)

	s = gs.GetSnowDepth(51.51, 9.37)
	sd, saw, icen = p2s.SnowDepthToXplaneSnowNow(s)
	logger.Infof("s = %0.2f, saw = %0.2f, icen = %0.2f", sd, saw, icen)

	s = gs.GetSnowDepth(51.418441, 9.42) // to the east
	sd, saw, icen = p2s.SnowDepthToXplaneSnowNow(s)
	logger.Infof("s = %0.2f, saw = %0.2f, icen = %0.2f", sd, saw, icen)

	s = gs.GetSnowDepth(51.5, 9.38)
	sd, saw, icen = p2s.SnowDepthToXplaneSnowNow(s)
	logger.Infof("s = %0.2f, saw = %0.2f, icen = %0.2f", sd, saw, icen)

	s = gs.GetSnowDepth(51.51, 9.38)
	sd, saw, icen = p2s.SnowDepthToXplaneSnowNow(s)
	logger.Infof("s = %0.2f, saw = %0.2f, icen = %0.2f", sd, saw, icen)

	fmt.Println("-----------------------------------------")
	s = gs.GetSnowDepth(51.49, 9.37)
	s = gs.GetSnowDepth(51.50, 9.37)
	s = gs.GetSnowDepth(51.51, 9.37)
}
