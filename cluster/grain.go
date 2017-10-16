package cluster

import "time"

type Grain struct {
	id string
}

func (g *Grain) ID() string {
	return g.id
}

func (g *Grain) Init(id string) {
	g.id = id
}

type GrainCallOption func(*GrainCallConfig)

type GrainCallConfig struct {
	RetryCount int
	Timeout    time.Duration
	RetryAction func(n int)
}

func DefaultGrainCallConfig() *GrainCallConfig {
	return &GrainCallConfig{
		RetryCount: 10,
		Timeout:    5 * time.Second,
		RetryAction: func(i int) {
			i++
			time.Sleep( time.Duration(i * i * 50))
		},
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
