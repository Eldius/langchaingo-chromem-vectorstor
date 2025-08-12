package chromem

import "log/slog"

type storageOptions struct {
	dbPath   string
	collName string
	logger   *slog.Logger
}

type Option func(*storageOptions)

func WithDBPath(dbPath string) Option {
	return func(o *storageOptions) {
		o.dbPath = dbPath
	}
}

func WithCollection(collName string) Option {
	return func(o *storageOptions) {
		o.collName = collName
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(o *storageOptions) {
		o.logger = logger
	}
}

func defaultStorageOptions() storageOptions {
	return storageOptions{
		dbPath:   ".db",
		logger:   slog.Default(),
		collName: "documents",
	}
}
