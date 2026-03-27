package logger

import (
	"backend/deps/xlog"
	"backend/src/common/configdoc"
)

func Init(cfg *configdoc.ConfigBase) {
	xlog.InitDefaultLoggerWithOptions(xlog.Options{
		FilePath:      cfg.Log.Path,
		Level:         cfg.Log.Level,
		Rotation:      cfg.Log.Rotation,
		MaxFileSizeMB: cfg.Log.MaxFileSizeMB,
		RetentionDays: cfg.Log.RetentionDays,
		Skip:          cfg.Log.Skip,
		Sync:          cfg.Log.Sync,
		StdOut:        cfg.Log.StdOut,
		FileOut:       cfg.Log.FileOut,
	})
}

func Close() {
	xlog.Close()
}
