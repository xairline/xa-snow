package services

import (
	"github.com/stretchr/testify/mock"
	"log"
	"os"
	"testing"
)

// MockLogger is a mock type for the Logger type
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string) {
	log.Printf(msg)
}

func (m *MockLogger) Debugf(format string, a ...interface{}) {
	log.Printf(format, a...)
}

func (m *MockLogger) Debug(msg string) {
	log.Printf(msg)
}

func (m *MockLogger) Error(msg string) {
	log.Printf(msg)
}

func (m *MockLogger) Warningf(format string, a ...interface{}) {
	log.Printf(format, a...)
}

func (m *MockLogger) Warning(msg string) {
	log.Printf(msg)
}

// Infof is a mock method for logger Infof
func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

// Errorf is a mock method for logger Errorf
func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

var (
	service 	GribService
	p2x			Phys2XPlane
	mockLogger	*MockLogger
)

func TestDownloadGribFile(t *testing.T) {
	os.Unsetenv("USE_SNOD_CSV")

	mockLogger = new(MockLogger)
	mockLogger.On("Infof", mock.Anything, mock.Anything).Return()
	mockLogger.On("Errorf", mock.Anything, mock.Anything).Return()

	service = NewGribService(mockLogger, ".", "bin", NewCoastService(mockLogger, ".."))
	p2x = NewPhys2XPlane(mockLogger)

	_, _, _ = service.DownloadAndProcessGribFile(true, 0, 0, 0)
	mockLogger.AssertCalled(t, "Infof", "Downloading GRIB file from %s", mock.Anything)
}

func TestWrap(t *testing.T) {

	// call at a few locations that wrap indices and are prone to range violations
	// just check whether it bombs
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(51, 0.1))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(51, -0.1))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(51, 0.1))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(51, -0.1))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(-50, 180))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(-50, -180))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(-50, 179.9))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(-50, -179.9))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(90, 179.9))
	p2x.SnowDepthToXplaneSnowNow(service.GetSnowDepth(-90, -179.9))
}
