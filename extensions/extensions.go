package extensions

import "sync/atomic"

type ExtensionId int32

var currentId int32 = 0

type Extension interface {
	Id() ExtensionId
}

type Extensions struct {
	extensions []Extension
}

func NewExtensions() *Extensions {
	ex := &Extensions{
		extensions: make([]Extension, 100),
	}
	return ex
}

func NextExtensionId() ExtensionId {
	id := atomic.AddInt32(&currentId, 1)
	return ExtensionId(id)
}

func (ex *Extensions) Get(id ExtensionId) Extension {
	return ex.extensions[id]
}

func (ex *Extensions) Register(extension Extension) {
	id := extension.Id()
	ex.extensions[id] = extension
}
