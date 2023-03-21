package eru_logs

import (
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var Logger *zap.Logger

func LogInit(serviceName string) {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logConfig := []byte(fmt.Sprint(`{
       "level" :"`, logLevel, `",
       "encoding": "json",
       "outputPaths":["stdout"],
       "errorOutputPaths":["stderr"],
 	   "initialFields": {"service": "`, serviceName, `"},
       "encoderConfig": {
           "messageKey":"msg",
           "levelKey":"level",
           "timeKey":"ts",
           "callerKey":"src",
           "levelEncoder":"lowercase"
       }
   }`))

	var zapConfig zap.Config

	if err := json.Unmarshal(logConfig, &zapConfig); err != nil {
		panic(err)
	}
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	var err error
	Logger, err = zapConfig.Build()

	if err != nil {
		panic(err)
	}
	defer Logger.Sync()
	Logger.With(zap.String("foo", "bar")).Info("logger initialized")
	Logger.Error("error example")
}

func NewContext(ctx context.Context, fields ...zap.Field) context.Context {
	return context.WithValue(ctx, "ctxlog", WithContext(ctx).With(fields...))
}

func WithContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Logger
	}
	if ctxLogger, ok := ctx.Value("ctxlog").(*zap.Logger); ok {
		return ctxLogger
	} else {
		return Logger
	}
}
