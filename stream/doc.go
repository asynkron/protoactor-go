// Package stream bridges actor messages with Go channels.
//
// The stream package provides utilities for converting actor message flows into Go channels,
// enabling integration between actor-based code and channel-based code. It supports both
// typed and untyped streams.
//
// Key types:
//   - TypedStream: Typed stream that converts actor messages of type T into a channel
//   - UntypedStream: Untyped stream that converts all actor messages into a channel
//
// Basic usage:
//
//	// Create a typed stream for string messages
//	stream := stream.NewTypedStream[string](system)
//
//	// Send messages to the stream's PID
//	system.Root.Send(stream.PID(), "hello")
//	system.Root.Send(stream.PID(), "world")
//
//	// Receive from the channel
//	for msg := range stream.C() {
//	    fmt.Println(msg)
//	}
//
//	// Close the stream when done
//	stream.Close()
package stream
