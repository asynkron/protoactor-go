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

type GrainCallOptions struct {
	RetryCount  int
	Timeout     time.Duration
	RetryAction func(n int)
}

var defaultGrainCallOptions *GrainCallOptions

func DefaultGrainCallOptions() *GrainCallOptions {
	if defaultGrainCallOptions == nil {
		defaultGrainCallOptions = NewGrainCallOptions()
	}
	return defaultGrainCallOptions
}

func NewGrainCallOptions() *GrainCallOptions {
	return &GrainCallOptions{
		RetryCount: 10,
		Timeout:    cfg.TimeoutTime,
		RetryAction: func(i int) {
			i++
			time.Sleep(time.Duration(i * i * 50))
		},
	}
}

func (config *GrainCallOptions) WithTimeout(timeout time.Duration) *GrainCallOptions {
	config.Timeout = timeout
	return config
}

func (config *GrainCallOptions) WithRetry(count int) *GrainCallOptions {
	config.RetryCount = count
	return config
}

func (config *GrainCallOptions) WithRetryAction(act func(i int)) *GrainCallOptions {
	config.RetryAction = act
	return config
}
