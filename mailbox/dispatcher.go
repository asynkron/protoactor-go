package mailbox

import "runtime"

type Dispatcher interface {
	Schedule(fn func())
	Throughput() int
	AfterStart()
	BeforeTerminate()
	BeforeBatchProcess()
	BeforeProcessingMessage()
}

type goroutineDispatcher struct {
	throughput int
	index      int
}

func (goroutineDispatcher) Schedule(fn func()) {
	go fn()
}

func (d goroutineDispatcher) AfterStart() {
}
func (d goroutineDispatcher) BeforeTerminate() {
}
func (d goroutineDispatcher) Throughput() int {
	return d.throughput
}

func (d goroutineDispatcher) BeforeProcessingMessage() {
	if d.index > d.throughput {
		d.index = 0
		runtime.Gosched()
	}
	d.index++
}

func (d goroutineDispatcher) BeforeBatchProcess() {
	d.index = 0
}

func NewDefaultDispatcher(throughput int) Dispatcher {
	return &goroutineDispatcher{throughput: throughput}
}
