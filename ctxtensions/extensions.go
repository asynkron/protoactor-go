package ctxtensions

import "sync/atomic"

type ExtensionID int32

var currentID int32

type Extension interface {
	ExtensionID() ExtensionID
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

func NextExtensionID() ExtensionID {
	id := atomic.AddInt32(&currentID, 1)
	return ExtensionID(id)
}

func (ex *Extensions) Get(id ExtensionID) Extension {
	return ex.extensions[id]
}

func (ex *Extensions) Set(extension Extension) {
	id := extension.ExtensionID()
	ex.extensions[id] = extension
}
