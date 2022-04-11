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
		extensions: make([]ContextExtension, 3),
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
	id := int32(extension.ExtensionID())
	if id >= int32(len(ex.extensions)) {
		newExtensions := make([]ContextExtension, id*2)
		copy(newExtensions, ex.extensions)
		ex.extensions = newExtensions
	}
	ex.extensions[id] = extension
}
