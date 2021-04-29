package actor

import "time"

type Config struct {
	DeadLetterThrottleInterval  time.Duration      //throttle deadletter logging after this interval
	DeadLetterThrottleCount     int                //throttle deadletter logging after this count
	DeadLetterRequestLogging    bool               //do not log deadletters with sender
	DeveloperSupervisionLogging bool               //console log and promote supervision logs to Warning level
	DiagnosticsSerializer       func(Actor) string //extract diagnostics from actor and return as string
}

func EmptyConfig() Config {
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

func (asc Config) WithDeadLetterThrottleInterval(duration time.Duration) Config {
	asc.DeadLetterThrottleInterval = duration
	return asc
}

func (asc Config) WithDeadLetterThrottleCount(count int) Config {
	asc.DeadLetterThrottleCount = count
	return asc
}

func (asc Config) WithDeadLetterRequestLogging(enabled bool) Config {
	asc.DeadLetterRequestLogging = enabled
	return asc
}

func (asc Config) WithDeveloperSupervisionLogging(enabled bool) Config {
	asc.DeveloperSupervisionLogging = enabled
	return asc
}

func (asc Config) WithDiagnosticsSerializer(serializer func(Actor) string) Config {
	asc.DiagnosticsSerializer = serializer
	return asc
}
