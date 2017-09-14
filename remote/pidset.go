package remote

import "github.com/AsynkronIT/protoactor-go/actor"

// set for watchee
type PIDSet struct {
	pids map[string]*actor.PID //pid cache for remote actor
}

func (wps *PIDSet) Add(pid *actor.PID) {
	if wps.pids == nil {
		wps.pids = make(map[string]*actor.PID)
	}
	wps.pids[pid.Id] = pid
}

func (wps *PIDSet) Get(id string) (pid *actor.PID, founded bool) {
	pid, founded = wps.pids[id]
	return
}

func (wps *PIDSet) Remove(id string) (pid *actor.PID, deleted bool) {
	pid, deleted = wps.pids[id]
	if deleted {
		delete(wps.pids, pid.Id)
		if len(wps.pids) == 0 {
			wps.pids = nil
		}
	}
	return
}
func (wps *PIDSet) All() []*actor.PID {
	var result []*actor.PID
	for _, pid := range wps.pids {
		result = append(result, pid)
	}
	return result
}
func (wps *PIDSet) Clean() {
	wps.pids = nil
}
func (wps *PIDSet) Size() int {
	if wps.pids == nil {
		return 0
	} else {
		return len(wps.pids)
	}

}
