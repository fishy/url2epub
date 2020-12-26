package set

// Interface defines a set of interface{}.
//
// The actual value put in must be comparable.
type Interface map[interface{}]Value

// Literal creates an Interface set from literals.
func Literal(literals ...interface{}) Interface {
	if len(literals) <= 0 {
		return nil
	}
	set := make(Interface, len(literals))
	for _, v := range literals {
		set.Add(v)
	}
	return set
}

// Add adds an value to the set.
func (s Interface) Add(v interface{}) {
	s[v] = DummyValue
}

// Has returns true if v is in the string set.
func (s Interface) Has(v interface{}) bool {
	_, ok := s[v]
	return ok
}
