package competition

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"strings"
)

func init() {
	Register(&Trivia{})
}

// Trivia implements the trivia competition type.
type Trivia struct{}

// Type returns the competition type name.
func (t *Trivia) Type() string { return "trivia" }

// Generate picks a random trivia question.
func (t *Trivia) Generate() (string, string, string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(TriviaQuestions))))
	if err != nil {
		return "", "", "", err
	}

	q := TriviaQuestions[n.Int64()]
	data, _ := json.Marshal(map[string]string{"question": q.Question, "answer": q.Answer})
	return q.Question, q.Answer, string(data), nil
}

// CheckAnswer does case-insensitive comparison with fuzzy tolerance.
func (t *Trivia) CheckAnswer(submitted, correct string) bool {
	submitted = strings.TrimSpace(strings.ToLower(submitted))
	correct = strings.TrimSpace(strings.ToLower(correct))

	if submitted == correct {
		return true
	}

	// Fuzzy: if correct answer is a single word, check if submitted contains it
	if !strings.Contains(correct, " ") && strings.Contains(submitted, correct) {
		return true
	}

	return false
}

// Description returns the competition description.
func (t *Trivia) Description() string {
	return "Answer the trivia question correctly to earn a shield!"
}
