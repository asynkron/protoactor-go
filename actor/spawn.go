package actor

// Spawn starts a new actor based on props and named with a unique id
// Deprecated: Use context.Spawn instead.
func Spawn(props *Props) *PID {
	return EmptyRootContext.Spawn(props)
}

// SpawnPrefix starts a new actor based on props and named using a prefix followed by a unique id
// Deprecated: Use context.SpawnPrefix instead.
func SpawnPrefix(props *Props, prefix string) *PID {
	return EmptyRootContext.SpawnPrefix(props, prefix)
}

// SpawnNamed starts a new actor based on props and named using the specified name
//
// If name exists, error will be ErrNameExists
// Deprecated: Use context.SpawnNamed instead.
func SpawnNamed(props *Props, name string) (*PID, error) {
	context := EmptyRootContext
	return context.SpawnNamed(props, name)
}
