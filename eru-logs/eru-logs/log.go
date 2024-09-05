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

//var FileLogger *zap.Logger

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
	undo := zap.ReplaceGlobals(Logger)
	defer undo()

	defer Logger.Sync()

	/*fileLogConfig := []byte(fmt.Sprint(`{
	       "level" :"`, logLevel, `",
	       "encoding": "json",
	       "outputPaths":["stdout","func_profile.log"],
	       "errorOutputPaths":["stderr"],
	 	   "initialFields": {"service": "`, serviceName, `"},
	       "encoderConfig": {
	           "messageKey":"msg",
	           "timeKey":"ts"
	       }
	   }`))

		var fileZapConfig zap.Config

		if err := json.Unmarshal(fileLogConfig, &fileZapConfig); err != nil {
			panic(err)
		}
		fileZapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")
		fileZapConfig.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

		var fileErr error
		FileLogger, fileErr = fileZapConfig.Build()
		if fileErr != nil {
			panic(fileErr)
		}
		fileundo := zap.ReplaceGlobals(FileLogger)
		defer fileundo()

		defer Logger.Sync()
		defer FileLogger.Sync()

	*/

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

func Sprintify(l ...interface{}) string {
	Logger.Info(fmt.Sprint(l))
	return fmt.Sprint(l)[0:1000]
}
