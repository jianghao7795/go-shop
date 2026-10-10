package logx

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.SugaredLogger

// Init 初始化日志：按日期分目录 logs/YYYY-MM-DD/，按级别分文件
// info.log（Info 及以上）、warning.log（Warn 及以上）、error.log（Error 及以上）、
// fatal.log（Fatal），同时输出到 stdout。
func Init() error {
	dir := filepath.Join("logs", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	infoFile, err := openLogFile(filepath.Join(dir, "info.log"))
	if err != nil {
		return err
	}
	warnFile, err := openLogFile(filepath.Join(dir, "warning.log"))
	if err != nil {
		return err
	}
	errFile, err := openLogFile(filepath.Join(dir, "error.log"))
	if err != nil {
		return err
	}
	fatalFile, err := openLogFile(filepath.Join(dir, "fatal.log"))
	if err != nil {
		return err
	}

	enc := consoleEncoder()

	core := zapcore.NewTee(
		zapcore.NewCore(enc, zapcore.AddSync(os.Stdout), zapcore.InfoLevel),
		zapcore.NewCore(enc, zapcore.AddSync(infoFile), zapcore.InfoLevel),
		zapcore.NewCore(enc, zapcore.AddSync(warnFile), zapcore.WarnLevel),
		zapcore.NewCore(enc, zapcore.AddSync(errFile), zapcore.ErrorLevel),
		zapcore.NewCore(enc, zapcore.AddSync(fatalFile), zapcore.FatalLevel),
	)

	logger = zap.New(core, zap.AddCaller()).Sugar()
	return nil
}

// openLogFile 以追加写模式打开日志文件。
func openLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// consoleEncoder 返回控制台可读的编码器配置。
func consoleEncoder() zapcore.Encoder {
	return zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	})
}

// L 返回全局 sugared logger；未调用 Init 时回退到开发默认配置。
func L() *zap.SugaredLogger {
	if logger == nil {
		l, _ := zap.NewDevelopment()
		logger = l.Sugar()
	}
	return logger
}
