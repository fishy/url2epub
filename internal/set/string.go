package set

// String defines a set of string.
type String map[string]Value

// StringLiteral creates a String set from string literals.
func StringLiteral(literals ...string) String {
	if len(literals) <= 0 {
		return nil
	}
	set := make(String, len(literals))
	for _, v := range literals {
		set.Add(v)
	}
	return set
}

// Add adds an value to the set.
func (s String) Add(v string) {
	s[v] = DummyValue
}

// Has returns true if v is in the string set.
func (s String) Has(v string) bool {
	_, ok := s[v]
	return ok
}
