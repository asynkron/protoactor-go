package ctxext

import "sync/atomic"

type ContextExtensionID int32

var currentContextExtensionID int32

type ContextExtension interface {
	ExtensionID() ContextExtensionID
}

type ContextExtensions struct {
	extensions []ContextExtension
}

func NewContextExtensions() *ContextExtensions {
	ex := &ContextExtensions{
		extensions: make([]ContextExtension, 100),
	}
	return ex
}

func NextContextExtensionID() ContextExtensionID {
	id := atomic.AddInt32(&currentContextExtensionID, 1)
	return ContextExtensionID(id)
}

func (ex *ContextExtensions) Get(id ContextExtensionID) ContextExtension {
	return ex.extensions[id]
}

func (ex *ContextExtensions) Set(extension ContextExtension) {
	id := extension.ExtensionID()
	ex.extensions[id] = extension
}
