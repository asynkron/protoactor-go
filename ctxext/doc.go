// Package ctxext provides context extension mechanisms for attaching custom data to actor contexts.
//
// The ctxext package enables extending actor contexts with custom functionality through a
// type-safe extension system. Each extension is identified by a unique ID and can be
// retrieved from the context when needed.
//
// Key types:
//   - ContextExtension: Interface for types that can be attached to actor contexts
//   - ContextExtensionID: Unique identifier for a context extension
//   - ContextExtensions: Collection for storing and retrieving extensions
//
// Key functions:
//   - NextContextExtensionID: Generates a new unique extension ID
//
// Basic usage:
//
//	// Define a custom extension
//	type MyExtension struct {
//	    id ctxext.ContextExtensionID
//	    data string
//	}
//
//	var myExtensionID = ctxext.NextContextExtensionID()
//
//	func (e *MyExtension) ExtensionID() ctxext.ContextExtensionID {
//	    return myExtensionID
//	}
//
//	// Attach to context (typically done during actor initialization)
//	ext := &MyExtension{data: "custom data"}
//	ctx.Extensions().Set(ext)
//
//	// Retrieve from context
//	retrieved := ctx.Extensions().Get(myExtensionID).(*MyExtension)
package ctxext
