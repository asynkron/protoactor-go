// Package ctxext provides extension mechanisms for actor contexts.
package ctxext

import "sync/atomic"

// ContextExtensionID uniquely identifies a ContextExtension.
type ContextExtensionID int32

var currentContextExtensionID int32

// ContextExtension describes an extension that can be attached to an actor Context.
type ContextExtension interface {
	ExtensionID() ContextExtensionID
}

// ContextExtensions stores registered ContextExtension values.
type ContextExtensions struct {
	extensions []ContextExtension
}

// NewContextExtensions creates an empty ContextExtensions collection.
func NewContextExtensions() *ContextExtensions {
	ex := &ContextExtensions{
		extensions: make([]ContextExtension, 3),
	}

	return ex
}

// NextContextExtensionID returns a new unique ContextExtensionID.
func NextContextExtensionID() ContextExtensionID {
	id := atomic.AddInt32(&currentContextExtensionID, 1)
	return ContextExtensionID(id)
}

// Get retrieves the ContextExtension associated with id.
func (ex *ContextExtensions) Get(id ContextExtensionID) ContextExtension {
	return ex.extensions[id]
}

// Set registers the extension in the collection, growing the slice if needed.
func (ex *ContextExtensions) Set(extension ContextExtension) {
	id := int32(extension.ExtensionID())
	if id >= int32(len(ex.extensions)) {
		newExtensions := make([]ContextExtension, id*2)
		copy(newExtensions, ex.extensions)
		ex.extensions = newExtensions
	}
	ex.extensions[id] = extension
}
