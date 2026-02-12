// Package plugin provides actor middleware and extension mechanisms.
//
// The plugin package enables extending actor behavior through middleware patterns.
// Plugins can intercept actor lifecycle events and messages to add cross-cutting
// concerns such as passivation, logging, or custom behavior.
//
// Key types:
//   - plugin: Interface for actor plugins that hook into lifecycle and message processing
//
// Key functions:
//   - Use: Converts a plugin into receiver middleware for use with actor Props
//
// Built-in plugins:
//   - PassivationPlugin: Automatically stops idle actors after a configured timeout
//
// Basic usage:
//
//	// Create a passivation plugin to stop actors after 30 seconds of inactivity
//	p := plugin.NewPassivationPlugin(30 * time.Second)
//
//	// Apply the plugin as middleware
//	props := actor.PropsFromFunc(myReceive).
//	    WithReceiverMiddleware(plugin.Use(p))
//
//	pid := system.Root.Spawn(props)
package plugin
