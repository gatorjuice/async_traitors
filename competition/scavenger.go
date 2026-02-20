package competition

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
)

func init() {
	Register(&Scavenger{})
}

// Scavenger implements the scavenger hunt competition type.
type Scavenger struct{}

// Type returns the competition type name.
func (sc *Scavenger) Type() string { return "scavenger" }

// Generate picks a random scavenger prompt.
func (sc *Scavenger) Generate() (string, string, string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(ScavengerPrompts))))
	if err != nil {
		return "", "", "", err
	}

	prompt := ScavengerPrompts[n.Int64()]
	data, _ := json.Marshal(map[string]string{"prompt": prompt})
	return prompt, "", string(data), nil
}

// CheckAnswer always returns true (honor system).
func (sc *Scavenger) CheckAnswer(_, _ string) bool {
	return true
}

// Description returns the competition description.
func (sc *Scavenger) Description() string {
	return "Complete the scavenger challenge and use /claim-shield if you did it!"
}
