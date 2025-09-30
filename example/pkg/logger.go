package pkg

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggerCfg struct {
	Development   bool
	DisableCaller bool
	DisableJson   bool
	Level         string
}

// Logger is a logger implementation using zap
type Logger struct {
	cfg         LoggerCfg
	sugarLogger *zap.SugaredLogger
}

// NewLogger creates a new logger instance
func NewLogger(cfg LoggerCfg) *Logger {
	logger := &Logger{cfg: cfg}
	logger.initLogger()
	return logger
}

// For mapping config logger to app logger levels
var loggerLevelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

func (l *Logger) getLoggerLevel() zapcore.Level {
	level, ok := loggerLevelMap[l.cfg.Level]
	if !ok {
		return zapcore.DebugLevel
	}

	return level
}

func (l *Logger) initLogger() {
	logLevel := l.getLoggerLevel()

	logWriter := zapcore.AddSync(os.Stdout)

	var encoderCfg zapcore.EncoderConfig
	if l.cfg.Development {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderCfg = zap.NewProductionEncoderConfig()
	}

	var encoder zapcore.Encoder
	encoderCfg.LevelKey = "LEVEL"
	encoderCfg.CallerKey = "CALLER"
	encoderCfg.TimeKey = "TIME"
	encoderCfg.NameKey = "NAME"
	encoderCfg.MessageKey = "MESSAGE"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	if l.cfg.DisableJson {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, logWriter, zap.NewAtomicLevelAt(logLevel))
	var callerArgs []zap.Option
	if !l.cfg.DisableCaller {
		callerArgs = append(callerArgs, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	logger := zap.New(core, callerArgs...)

	l.sugarLogger = logger.Sugar()
}

// Logger methods

func (l *Logger) Debug(args ...interface{}) {
	l.sugarLogger.Debug(args...)
}

func (l *Logger) Debugf(template string, args ...interface{}) {
	l.sugarLogger.Debugf(template, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.sugarLogger.Info(args...)
}

func (l *Logger) Infof(template string, args ...interface{}) {
	l.sugarLogger.Infof(template, args...)
}

func (l *Logger) Warn(args ...interface{}) {
	l.sugarLogger.Warn(args...)
}

func (l *Logger) Warnf(template string, args ...interface{}) {
	l.sugarLogger.Warnf(template, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.sugarLogger.Error(args...)
}

func (l *Logger) Errorf(template string, args ...interface{}) {
	l.sugarLogger.Errorf(template, args...)
}

func (l *Logger) DPanic(args ...interface{}) {
	l.sugarLogger.DPanic(args...)
}

func (l *Logger) DPanicf(template string, args ...interface{}) {
	l.sugarLogger.DPanicf(template, args...)
}

func (l *Logger) Panic(args ...interface{}) {
	l.sugarLogger.Panic(args...)
}

func (l *Logger) Panicf(template string, args ...interface{}) {
	l.sugarLogger.Panicf(template, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.sugarLogger.Fatal(args...)
}

func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.sugarLogger.Fatalf(template, args...)
}
