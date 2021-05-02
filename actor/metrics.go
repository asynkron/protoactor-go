package actor

import "github.com/AsynkronIT/protoactor-go/extensions"

var extensionId = extensions.NextExtensionId()

type Metrics struct {
	enabled bool
}

func (m *Metrics) Enabled() bool {
	return m.enabled
}
func (m *Metrics) Id() extensions.ExtensionId {
	return extensionId
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

//func (m *Metrics) NewGauge() {
//
//}
//
//func (m *Metrics) NewCounter() {
//
//}
//
//func (m *Metrics) NewHistogram() {
//
//}
