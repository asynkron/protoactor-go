package actor

import "github.com/AsynkronIT/protoactor-go/extensions"

var extensionId = extensions.NextExtensionId()

type Metrics struct {
}

func (m *Metrics) Id() extensions.ExtensionId {
	return extensionId
}

func NewMetrics() *Metrics {
	return &Metrics{}
}
