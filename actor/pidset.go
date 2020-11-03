package actor

const pidSetSliceLen = 16

type PIDSet struct {
	pids []*PID
}

// NewPIDSet returns a new PIDSet with the given pids.
func NewPIDSet(pids ...*PID) *PIDSet {
	var s PIDSet
	for _, pid := range pids {
		s.Add(pid)
	}
	return &s
}

func (p *PIDSet) indexOf(v *PID) int {
	for i, pid := range p.pids {
		if v.Equal(pid) {
			return i
		}
	}
	return -1
}

// Add adds the element v to the set
func (p *PIDSet) Add(v *PID) {
	if p.indexOf(v) != -1 {
		return
	}
	p.pids = append(p.pids, v)
}

// Remove removes v from the set and returns true if them element existed
func (p *PIDSet) Remove(v *PID) bool {
	i := p.indexOf(v)
	if i == -1 {
		return false
	}

	p.pids = append(p.pids[:i], p.pids[i+1:]...)

	return true
}

// Contains reports whether v is an element of the set
func (p *PIDSet) Contains(v *PID) bool {
	return p.indexOf(v) != -1
}

// Len returns the number of elements in the set
func (p *PIDSet) Len() int {
	return len(p.pids)
}

// Clear removes all the elements in the set
func (p *PIDSet) Clear() {
	p.pids = p.pids[:0]
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
	var s PIDSet
	s.pids = make([]*PID, len(p.pids))
	for i, v := range p.pids {
		s.pids[i] = v
	}
	return &s
}
