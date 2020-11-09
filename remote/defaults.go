package remote

import "github.com/AsynkronIT/protoactor-go/actor"

var (
	_default *Remote
)

// Start default remote intance
func Start(config Config) {
	_default = NewRemote(actor.System, config)
	_default.Start()
}

// Shutdown default remote intance
func Shutdown(graceful bool) {
	if _default == nil {
		plog.Error("default instance was nil")
		return
	}
	_default.Shutdown(graceful)
	_default = nil

}
