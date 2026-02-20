package game

import (
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/gatorjuice/async_traitors/db"
	_ "modernc.org/sqlite"
)

var testGameCounter atomic.Int64

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(on)")
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	if err := db.RunMigrations(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func createTestGame(t *testing.T, database *sql.DB, numPlayers int) int64 {
	t.Helper()
	code := fmt.Sprintf("T%d", testGameCounter.Add(1))
	gameID, err := db.CreateGame(database, code, "guild", "chan", "admin")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < numPlayers; i++ {
		pid := fmt.Sprintf("player%c_%d", rune('A'+i), gameID)
		name := fmt.Sprintf("Player%c", rune('A'+i))
		if err := db.AddPlayer(database, gameID, pid, name); err != nil {
			t.Fatal(err)
		}
	}
	return gameID
}

func getPlayerIDs(t *testing.T, database *sql.DB, gameID int64) []string {
	t.Helper()
	players, err := db.GetAllPlayers(database, gameID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(players))
	for i, p := range players {
		ids[i] = p.DiscordID
	}
	return ids
}
