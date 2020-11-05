package actor

import "fmt"

type PIDSet struct {
	pids   []*PID
	lookup map[string]*PID
}

func (p *PIDSet) key(pid *PID) string {
	return fmt.Sprintf("%v:%v", pid.Address, pid.Id)
}

// NewPIDSet returns a new PIDSet with the given pids.
func NewPIDSet(pids ...*PID) *PIDSet {
	p := &PIDSet{}
	for _, pid := range pids {
		p.Add(pid)
	}
	return p
}

func (p *PIDSet) ensureInit() {
	if p.lookup == nil {
		p.lookup = make(map[string]*PID)
	}
}

func (p *PIDSet) indexOf(v *PID) int {
	for i, pid := range p.pids {
		if v.Equal(pid) {
			return i
		}
	}
	return -1
}

func (p *PIDSet) Contains(v *PID) bool {
	return p.lookup[p.key(v)] != nil
}

// Add adds the element v to the set
func (p *PIDSet) Add(v *PID) {
	p.ensureInit()
	if p.Contains(v) {
		return
	}
	p.lookup[p.key(v)] = v
	p.pids = append(p.pids, v)
}

// Remove removes v from the set and returns true if them element existed
func (p *PIDSet) Remove(v *PID) bool {
	p.ensureInit()
	i := p.indexOf(v)
	if i == -1 {
		return false
	}

	delete(p.lookup, p.key(v))

	p.pids = append(p.pids[:i], p.pids[i+1:]...)

	return true
}

// Len returns the number of elements in the set
func (p *PIDSet) Len() int {
	return len(p.pids)
}

// Clear removes all the elements in the set
func (p *PIDSet) Clear() {
	p.pids = p.pids[:0]
	p.lookup = make(map[string]*PID)
}

// Empty reports whether the set is empty
func (p *PIDSet) Empty() bool {
	return p.Len() == 0
}

// Values returns all the elements of the set as a slice
func (p *PIDSet) Values() []*PID {
	return p.pids
}

// ForEach invokes f for every element of the set
func (p *PIDSet) ForEach(f func(i int, pid *PID)) {
	for i, pid := range p.pids {
		f(i, pid)
	}
}

func (p *PIDSet) Get(index int) *PID {
	return p.pids[index]
}

func (p *PIDSet) Clone() *PIDSet {
	return NewPIDSet(p.pids...)
}
