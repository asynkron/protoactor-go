package actor

type PIDSet struct {
	pids   []*PID
	lookup map[pidKey]int
}

// pidKey is used as a key in the lookup map to avoid allocations.
type pidKey struct {
	address string
	id      string
}

func (p *PIDSet) key(pid *PID) pidKey {
	return pidKey{address: pid.Address, id: pid.Id}
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
		p.lookup = make(map[pidKey]int)
	}
}

func (p *PIDSet) indexOf(v *PID) int {
	if idx, ok := p.lookup[p.key(v)]; ok {
		return idx
	}

	return -1
}

func (p *PIDSet) Contains(v *PID) bool {
	_, ok := p.lookup[p.key(v)]
	return ok
}

// Add adds the element v to the set.
func (p *PIDSet) Add(v *PID) {
	p.ensureInit()
	if p.Contains(v) {
		return
	}

	p.pids = append(p.pids, v)
	p.lookup[p.key(v)] = len(p.pids) - 1
}

// Remove removes v from the set and returns true if them element existed.
func (p *PIDSet) Remove(v *PID) bool {
	p.ensureInit()
	i := p.indexOf(v)
	if i == -1 {
		return false
	}

	delete(p.lookup, p.key(v))
	if i < len(p.pids)-1 {
		lastPID := p.pids[len(p.pids)-1]

		p.pids[i] = lastPID
		p.lookup[p.key(lastPID)] = i
	}

	p.pids = p.pids[:len(p.pids)-1]

	return true
}

// Len returns the number of elements in the set.
func (p *PIDSet) Len() int {
	return len(p.pids)
}

// Clear removes all the elements in the set.
func (p *PIDSet) Clear() {
	p.pids = p.pids[:0]
	p.lookup = make(map[pidKey]int)
}

// Empty reports whether the set is empty.
func (p *PIDSet) Empty() bool {
	return p.Len() == 0
}

// Values returns all the elements of the set as a slice.
func (p *PIDSet) Values() []*PID {
	return p.pids
}

// ForEach invokes f for every element of the set.
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
