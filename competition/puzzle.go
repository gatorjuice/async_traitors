package competition

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"strings"
)

func init() {
	Register(&Puzzle{})
}

// Puzzle implements the word scramble competition type.
type Puzzle struct{}

// Type returns the competition type name.
func (p *Puzzle) Type() string { return "puzzle" }

// Generate picks a random word and scrambles its letters.
func (p *Puzzle) Generate() (string, string, string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(PuzzleWords))))
	if err != nil {
		return "", "", "", err
	}

	word := PuzzleWords[n.Int64()]

	// Scramble letters using Fisher-Yates
	letters := []byte(word)
	for i := len(letters) - 1; i > 0; i-- {
		j, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		letters[i], letters[j.Int64()] = letters[j.Int64()], letters[i]
	}

	scrambled := string(letters)
	// Ensure scrambled != original
	if scrambled == word {
		letters[0], letters[len(letters)-1] = letters[len(letters)-1], letters[0]
		scrambled = string(letters)
	}

	question := "Unscramble this word: **" + scrambled + "**"
	data, _ := json.Marshal(map[string]string{"scrambled": scrambled, "answer": word})
	return question, word, string(data), nil
}

// CheckAnswer does case-insensitive exact match.
func (p *Puzzle) CheckAnswer(submitted, correct string) bool {
	return strings.EqualFold(strings.TrimSpace(submitted), strings.TrimSpace(correct))
}

// Description returns the competition description.
func (p *Puzzle) Description() string {
	return "Unscramble the word! First correct answer wins a shield."
}
