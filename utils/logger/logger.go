package logger

type Logger interface {
	Infof(format string, a ...interface{})
	Info(msg string)
	Debugf(format string, a ...interface{})
	Debug(msg string)
	Errorf(format string, a ...interface{})
	Error(msg string)
	Warningf(format string, a ...interface{})
	Warning(msg string)
}
