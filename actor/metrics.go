package actor

import "github.com/AsynkronIT/protoactor-go/extensions"

var extensionId = extensions.NextExtensionID()

type Metrics struct {
	enabled bool
}

func (m *Metrics) Enabled() bool {
	return m.enabled
}
func (m *Metrics) ExtensionID() extensions.ExtensionID {
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
