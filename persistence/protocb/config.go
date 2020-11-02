package protocb

type couchbaseConfig struct {
	async            bool
	snapshotInterval int
}

type CouchbaseOption func(*couchbaseConfig)

func WithAsync() CouchbaseOption {
	return func(config *couchbaseConfig) {
		config.async = true
	}
}

func WithSnapshot(interval int) CouchbaseOption {
	return func(config *couchbaseConfig) {
		config.snapshotInterval = interval
	}
}
