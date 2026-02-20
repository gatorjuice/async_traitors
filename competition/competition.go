package competition

// Competition defines the interface for competition types.
type Competition interface {
	Type() string
	Generate() (question string, answer string, data string, err error)
	CheckAnswer(submitted, correct string) bool
	Description() string
}

// Registry holds all registered competition types.
var Registry = map[string]Competition{}

// Register adds a competition type to the registry.
func Register(c Competition) {
	Registry[c.Type()] = c
}

// Get retrieves a competition type from the registry.
func Get(compType string) (Competition, bool) {
	c, ok := Registry[compType]
	return c, ok
}
