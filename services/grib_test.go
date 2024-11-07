package services

import (
	"github.com/stretchr/testify/mock"
	"log"
	"testing"
	"os"
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

func TestDownloadGribFile(t *testing.T) {
	os.Unsetenv("USE_ICEC_CSV")
	os.Unsetenv("USE_SNOD_CSV")

	mockLogger := new(MockLogger)
	mockLogger.On("Infof", mock.Anything, mock.Anything).Return()
	mockLogger.On("Errorf", mock.Anything, mock.Anything).Return()

	service := NewGribService(mockLogger, ".", "bin")
	_ = service.DownloadAndProcessGribFile()
	SnowDepthToXplaneSnowNow(service.GetSnowDepth(45.325356, -75.672249))
	mockLogger.AssertCalled(t, "Infof", "Downloading GRIB file from %s", mock.Anything)
}
