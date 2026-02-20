package competition

// TriviaQuestion holds a trivia question and answer.
type TriviaQuestion struct {
	Question string
	Answer   string
}

// TriviaQuestions is the seed bank of trivia questions.
var TriviaQuestions = []TriviaQuestion{
	{"What planet is known as the Red Planet?", "Mars"},
	{"How many sides does a hexagon have?", "6"},
	{"What is the chemical symbol for gold?", "Au"},
	{"What is the largest ocean on Earth?", "Pacific"},
	{"In what year did the Titanic sink?", "1912"},
	{"What is the smallest prime number?", "2"},
	{"What element does 'O' represent on the periodic table?", "Oxygen"},
	{"How many continents are there?", "7"},
	{"What is the capital of Japan?", "Tokyo"},
	{"What gas do plants absorb from the atmosphere?", "Carbon dioxide"},
	{"How many legs does a spider have?", "8"},
	{"What is the hardest natural substance on Earth?", "Diamond"},
	{"What planet is closest to the Sun?", "Mercury"},
	{"What is the longest river in the world?", "Nile"},
	{"What is the square root of 144?", "12"},
	{"What country has the most people?", "India"},
	{"What is the boiling point of water in Celsius?", "100"},
	{"How many strings does a standard guitar have?", "6"},
	{"What is the largest mammal in the world?", "Blue whale"},
	{"What year did World War II end?", "1945"},
	{"What is the currency of the United Kingdom?", "Pound"},
	{"How many bones are in the adult human body?", "206"},
}

// PuzzleWords is the seed bank for word scramble puzzles.
var PuzzleWords = []string{
	"ALGORITHM",
	"BUTTERFLY",
	"DINOSAUR",
	"ELEPHANT",
	"FREQUENCY",
	"GORGEOUS",
	"HARMONY",
	"ILLUSION",
	"JUNCTION",
	"KNOWLEDGE",
	"LABYRINTH",
	"MOUNTAIN",
	"NOTEBOOK",
	"PLATINUM",
	"QUESTION",
}

// ScavengerPrompts is the seed bank for scavenger hunt challenges.
var ScavengerPrompts = []string{
	"Find something blue in your house",
	"Take a photo of a book published before 2000",
	"Find an item that starts with the letter of your first name",
	"Locate something with more than 4 colors on it",
	"Find a kitchen utensil you rarely use",
	"Take a photo of something that makes you happy",
	"Find an object older than you are",
	"Locate something made of wood",
	"Find a piece of clothing you forgot you owned",
	"Take a photo of the view from your nearest window",
}
