package log

import (
	"context"
	"io"
	"sync"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	once   sync.Once
	logger *zap.SugaredLogger
)

type LoggerConfig struct {
	Namespace string `json:"namespace"`   //命名空间
	Project   string `json:"project"`     //项目名称
	Level     string `json:"level"`       //日志等级
	OutPutDir string `json:"out_put_dir"` //输出的目录
	Filename  string `json:"filename"`    //指定生成的文件
}

// Init 初始化日志
func Init(conf *LoggerConfig) {
	once.Do(func() {
		logger = newLogger(conf)
	})
}

func newLogger(conf *LoggerConfig) *zap.SugaredLogger {
	encoder := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "log",
		CallerKey:     "file",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05"))
		},
		EncodeName: zapcore.FullNameEncoder,
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		},
	})

	// 实现两个判断日志等级的interface
	infoLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return true
	})
	warnLevel := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.WarnLevel
	})
	infoHook := getWriter(conf.OutPutDir, conf.Filename)
	// 最后创建具体的Logger
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(ZapLevel(conf.Level))
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, zapcore.AddSync(infoHook), infoLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(infoHook), warnLevel),
	)
	logs := zap.New(core, zap.AddCaller(), zap.Development(), zap.AddCallerSkip(0)).Sugar()
	logs.With("namespace", conf.Namespace, "project", conf.Project)
	return logs
}

func getWriter(outputDir, filename string) io.Writer {
	// 生成rotatelogs的Logger 实际生成的文件名 demo.log.YYmmddHH
	// demo.log是指向最新日志的链接
	// 保存7天内的日志，每1小时(整点)分割一次日志
	hook, err := rotatelogs.New(
		// 没有使用go风格反人类的format格式
		outputDir+"%Y-%m-%d"+filename,
		rotatelogs.WithLinkName(filename),
		rotatelogs.WithMaxAge(time.Hour*24*7),
		rotatelogs.WithRotationTime(time.Hour*24),
	)
	if err != nil {
		panic(err)
	}
	return hook
}

var zapLevelMap = map[string]zapcore.Level{
	"debug":  zapcore.DebugLevel,
	"info":   zapcore.InfoLevel,
	"warn":   zapcore.WarnLevel,
	"error":  zapcore.ErrorLevel,
	"dpanic": zapcore.DPanicLevel,
	"panic":  zapcore.PanicLevel,
	"fatal":  zapcore.FatalLevel,
}

// ZapLevel 未知 level 则返回 InfoLevel
func ZapLevel(level string) zapcore.Level {
	if l, ok := zapLevelMap[level]; ok {
		return l
	}
	return zap.InfoLevel
}

// Logger 获取全局logger 对象
func Logger() *zap.SugaredLogger {
	if logger == nil {
		panic("nil logger")
	}

	return logger
}

const loggerCtxKey = "Ctx-Key-Logger"

func LoggerCtxKey() string {
	return loggerCtxKey
}

// FromContext 日志上下文承接
func FromContext(ctx context.Context) *zap.SugaredLogger {
	if ctx == nil {
		panic("nil ctx")
	}

	val := ctx.Value(LoggerCtxKey())
	if v, ok := val.(*zap.SugaredLogger); ok {
		return v
	}

	return Logger()
}
