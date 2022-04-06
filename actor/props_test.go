package actor

import "testing"

func TestProps_Clone(t *testing.T) {
	p := PropsFromFunc(func(c Context) {}, WithOnInit(func(c Context) {}))
	p2 := p.Clone()
	if p == p2 {
		t.Error("Clone should return a new instance")
	}
}
