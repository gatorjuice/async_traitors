package competition

import (
	"strconv"
	"testing"
)

func TestRegistryHasAllTypes(t *testing.T) {
	expected := []string{"trivia", "speed", "puzzle", "scavenger"}
	for _, typ := range expected {
		if _, ok := Get(typ); !ok {
			t.Errorf("expected %s in registry", typ)
		}
	}
}

func TestTriviaGenerate(t *testing.T) {
	trivia := &Trivia{}
	q, a, _, err := trivia.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if q == "" {
		t.Error("expected non-empty question")
	}
	if a == "" {
		t.Error("expected non-empty answer")
	}
}

func TestTriviaCheckAnswer(t *testing.T) {
	trivia := &Trivia{}
	if !trivia.CheckAnswer("Mars", "Mars") {
		t.Error("exact match should be correct")
	}
	if !trivia.CheckAnswer("mars", "Mars") {
		t.Error("case-insensitive should be correct")
	}
	if !trivia.CheckAnswer("  Mars  ", "Mars") {
		t.Error("trimmed should be correct")
	}
	if trivia.CheckAnswer("Venus", "Mars") {
		t.Error("wrong answer should be incorrect")
	}
}

func TestSpeedGenerate(t *testing.T) {
	speed := &Speed{}
	q, a, _, err := speed.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if q == "" {
		t.Error("expected non-empty question")
	}
	// Answer should be a valid number
	if _, err := strconv.Atoi(a); err != nil {
		t.Errorf("expected numeric answer, got %s", a)
	}
}

func TestSpeedCheckAnswer(t *testing.T) {
	speed := &Speed{}
	if !speed.CheckAnswer("42", "42") {
		t.Error("correct answer should match")
	}
	if speed.CheckAnswer("43", "42") {
		t.Error("wrong answer should not match")
	}
}

func TestPuzzleGenerate(t *testing.T) {
	puzzle := &Puzzle{}
	q, a, _, err := puzzle.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if q == "" {
		t.Error("expected non-empty question")
	}
	if a == "" {
		t.Error("expected non-empty answer")
	}
}

func TestPuzzleCheckAnswer(t *testing.T) {
	puzzle := &Puzzle{}
	if !puzzle.CheckAnswer("ALGORITHM", "ALGORITHM") {
		t.Error("exact match should be correct")
	}
	if !puzzle.CheckAnswer("algorithm", "ALGORITHM") {
		t.Error("case-insensitive should be correct")
	}
	if puzzle.CheckAnswer("WRONG", "ALGORITHM") {
		t.Error("wrong answer should be incorrect")
	}
}

func TestScavengerGenerate(t *testing.T) {
	scav := &Scavenger{}
	q, _, _, err := scav.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if q == "" {
		t.Error("expected non-empty prompt")
	}
}

func TestScavengerCheckAnswer(t *testing.T) {
	scav := &Scavenger{}
	if !scav.CheckAnswer("anything", "") {
		t.Error("scavenger should always return true")
	}
	if !scav.CheckAnswer("", "") {
		t.Error("scavenger should always return true even with empty answer")
	}
}
