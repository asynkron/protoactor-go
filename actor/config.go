package actor

import "time"

type Config struct {
	DeadLetterThrottleInterval  time.Duration      //throttle deadletter logging after this interval
	DeadLetterThrottleCount     int32              //throttle deadletter logging after this count
	DeadLetterRequestLogging    bool               //do not log deadletters with sender
	DeveloperSupervisionLogging bool               //console log and promote supervision logs to Warning level
	DiagnosticsSerializer       func(Actor) string //extract diagnostics from actor and return as string
}

func defaultConfig() Config {
	return Config{
		DeadLetterThrottleInterval:  time.Duration(0),
		DeadLetterThrottleCount:     0,
		DeadLetterRequestLogging:    true,
		DeveloperSupervisionLogging: false,
		DiagnosticsSerializer: func(actor Actor) string {
			return ""
		},
	}
}

type ConfigOption func(config Config) Config

func Configure(options ...ConfigOption) Config {
	config := defaultConfig()
	for _, option := range options {
		config = option(config)
	}
	return config
}

func WithDeadLetterThrottleInterval(duration time.Duration) ConfigOption {
	return func(config Config) Config {
		config.DeadLetterThrottleInterval = duration
		return config
	}
}

func WithDeadLetterThrottleCount(count int32) ConfigOption {
	return func(config Config) Config {
		config.DeadLetterThrottleCount = count
		return config
	}
}

func WithDeadLetterRequestLogging(enabled bool) ConfigOption {
	return func(config Config) Config {
		config.DeadLetterRequestLogging = enabled
		return config
	}
}

func WithDeveloperSupervisionLogging(enabled bool) ConfigOption {
	return func(config Config) Config {
		config.DeveloperSupervisionLogging = enabled
		return config
	}
}

func WithDiagnosticsSerializer(serializer func(Actor) string) ConfigOption {
	return func(config Config) Config {
		config.DiagnosticsSerializer = serializer
		return config
	}
}
