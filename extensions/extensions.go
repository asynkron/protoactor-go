// Package extensions provides registration and lookup of actor extensions.
package extensions

import "sync/atomic"

// ExtensionID uniquely identifies a registered Extension.
type ExtensionID int32

var currentID int32 //nolint:gochecknoglobals

// Extension defines a feature that can be plugged into the actor system.
type Extension interface {
	ExtensionID() ExtensionID
}

// Extensions holds registered Extension instances.
type Extensions struct {
	extensions []Extension
}

// NewExtensions creates an empty Extensions collection.
func NewExtensions() *Extensions {
	ex := &Extensions{
		extensions: make([]Extension, 100),
	}

	return ex
}

// NextExtensionID returns a new unique ExtensionID.
func NextExtensionID() ExtensionID {
	id := atomic.AddInt32(&currentID, 1)

	return ExtensionID(id)
}

// Get retrieves the Extension associated with the given id.
func (ex *Extensions) Get(id ExtensionID) Extension {
	return ex.extensions[id]
}

// Register stores the extension in the collection using its ExtensionID.
func (ex *Extensions) Register(extension Extension) {
	id := extension.ExtensionID()
	ex.extensions[id] = extension
}
