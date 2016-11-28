package grain

import "time"

type GrainCallOption func(*GrainCallConfig)

type GrainCallConfig struct {
	RetryCount int
	Timeout    time.Duration
}

func DefaultGrainCallConfig() *GrainCallConfig {
	return &GrainCallConfig{
		RetryCount: 1,
		Timeout:    5 * time.Second,
	}
}

func ApplyGrainCallOptions(options []GrainCallOption) *GrainCallConfig {
	config := DefaultGrainCallConfig()
	for _, o := range options {
		o(config)
	}
	return config
}

func WithTimeout(timeout time.Duration) GrainCallOption {
	return func(o *GrainCallConfig) {
		o.Timeout = timeout
	}
}

func WithRetry(count int) GrainCallOption {
	return func(o *GrainCallConfig) {
		o.RetryCount = count
	}
}
