package cheapset

import (
	"github.com/emirpasic/gods/sets/hashset"
)

type Set struct {
	value interface{}
	set   *hashset.Set
}

func New() *Set {
	return &Set{}
}

func (s *Set) Add(value interface{}) {
	if s.set != nil {
		s.set.Add(value)
		return
	}

	if s.value != nil {
		s.set = hashset.New()
		s.set.Add(s.value)
		s.set.Add(value)
		s.value = nil
		return
	}

	s.value = value
}

func (s *Set) Remove(value interface{}) {
	if s.set != nil {
		s.set.Remove(value)
		return
	}

	if s.value == value {
		s.value = nil
	}
}

func (s *Set) Empty() bool {
	if s.set != nil {
		return s.set.Empty()
	}
	return s.value == nil
}

func (s *Set) Values() []interface{} {
	if s.set != nil {
		return s.set.Values()
	}

	if s.value != nil {
		return []interface{}{s.value}
	}

	return []interface{}{}
}
