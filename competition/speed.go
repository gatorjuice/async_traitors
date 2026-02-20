package competition

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

func init() {
	Register(&Speed{})
}

// Speed implements the speed math competition type.
type Speed struct{}

// Type returns the competition type name.
func (sp *Speed) Type() string { return "speed" }

// Generate produces a random math problem.
func (sp *Speed) Generate() (string, string, string, error) {
	ops := []struct {
		symbol string
		fn     func(int, int) int
	}{
		{"+", func(a, b int) int { return a + b }},
		{"-", func(a, b int) int { return a - b }},
		{"x", func(a, b int) int { return a * b }},
	}

	opIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(ops))))
	op := ops[opIdx.Int64()]

	var a, b int
	if op.symbol == "x" {
		an, _ := rand.Int(rand.Reader, big.NewInt(20))
		bn, _ := rand.Int(rand.Reader, big.NewInt(20))
		a = int(an.Int64()) + 2
		b = int(bn.Int64()) + 2
	} else {
		an, _ := rand.Int(rand.Reader, big.NewInt(900))
		bn, _ := rand.Int(rand.Reader, big.NewInt(900))
		a = int(an.Int64()) + 100
		b = int(bn.Int64()) + 100
	}

	result := op.fn(a, b)
	question := fmt.Sprintf("What is %d %s %d?", a, op.symbol, b)
	answer := fmt.Sprintf("%d", result)

	data, _ := json.Marshal(map[string]string{"question": question, "answer": answer})
	return question, answer, string(data), nil
}

// CheckAnswer checks for exact numeric match.
func (sp *Speed) CheckAnswer(submitted, correct string) bool {
	return strings.TrimSpace(submitted) == strings.TrimSpace(correct)
}

// Description returns the competition description.
func (sp *Speed) Description() string {
	return "Solve the math problem as fast as possible! Fastest correct answer wins a shield."
}
