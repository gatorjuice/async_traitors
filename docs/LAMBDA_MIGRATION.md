# EC2 → Lambda Migration Guide

Complete step-by-step guide for migrating Async Traitors from EC2 (Docker + SQLite + WebSocket gateway) to Lambda (API Gateway + DynamoDB + EventBridge). Estimated result: ~$0/month (free tier).

---

## Table of Contents

- [Phase 0: Interface Extraction](#phase-0-interface-extraction)
- [Phase 1: Discord HTTP Interactions Endpoint](#phase-1-discord-http-interactions-endpoint)
- [Phase 2: DynamoDB Store Implementation](#phase-2-dynamodb-store-implementation)
- [Phase 3: EventBridge Timer System](#phase-3-eventbridge-timer-system)
- [Phase 4: SAM Template & AWS Infrastructure](#phase-4-sam-template--aws-infrastructure)
- [Phase 5: Slash Command Registration](#phase-5-slash-command-registration)
- [Phase 6: CI/CD Pipeline](#phase-6-cicd-pipeline)
- [Phase 7: Data Migration & Cutover](#phase-7-data-migration--cutover)
- [Phase 8: EC2 Decommission](#phase-8-ec2-decommission)

---

## Phase 0: Interface Extraction

**Goal:** Decouple game logic from concrete `*sql.DB` and `*TimerManager` types so both SQLite and DynamoDB backends can be used interchangeably.

**Verification:** `go test ./...` passes. Deploy to EC2 works identically.

### 0.1 — Create `store/models.go`

Move model structs out of `db/` into a new `store/` package so both backends share them. The `db/` package currently defines these structs:

| Current Location | Struct |
|---|---|
| `db/games.go:9-32` | `Game` |
| `db/players.go:9-21` | `Player` |
| `db/votes.go:9-17` | `Vote` |
| `db/competitions.go:9-19` | `Competition` |
| `db/competitions.go:21-30` | `CompetitionResult` |
| `db/competitions.go:32-41` | `ShieldLogEntry` |
| `db/payments.go:9-15` | `Payment` |

Create `store/models.go`:

```go
package store

import "time"

type Game struct {
    ID                     int64
    JoinCode               string
    GuildID                string
    ChannelID              string
    CreatedBy              string
    Status                 string
    CurrentPhase           string
    CurrentRound           int
    TraitorThreadID        string
    TimerBreakfastMinutes  int
    TimerRoundtableMinutes int
    TimerNightMinutes      int
    TimerMissionMinutes    int
    RevealThreshold        int
    RecruitmentPending     bool
    HiatusStart            string
    HiatusEnd              string
    HiatusTimezone         string
    BuyinAmount            int
    EndBy                  string
    CreatedAt              time.Time
    UpdatedAt              time.Time
}

type Player struct {
    ID             int64
    GameID         int64
    DiscordID      string
    DiscordName    string
    Role           string
    Status         string
    HasShield      bool
    StatusRound    int
    RecruitedRound int
    WalletInfo     string
    JoinedAt       time.Time
}

type Vote struct {
    ID              int64
    GameID          int64
    Round           int
    Phase           string
    VoterDiscordID  string
    TargetDiscordID string
    CreatedAt       time.Time
}

type Competition struct {
    ID           int64
    GameID       int64
    Round        int
    CompType     string
    QuestionData string
    Answer       string
    Status       string
    CreatedAt    time.Time
}

type CompetitionResult struct {
    ID              int64
    CompetitionID   int64
    PlayerDiscordID string
    Answer          string
    Correct         bool
    TimeMs          int64
    SubmittedAt     time.Time
}

type ShieldLogEntry struct {
    ID              int64
    GameID          int64
    PlayerDiscordID string
    Source          string
    RoundGranted    int
    RoundUsed       *int
    CreatedAt       time.Time
}

type Payment struct {
    ID              int64
    GameID          int64
    WinnerDiscordID string
    LoserDiscordID  string
    CreatedAt       time.Time
}
```

After creating this file, update `db/*.go` to make the existing structs type aliases or remove them and import from `store/`. The simplest approach: **keep `db/` structs as-is for now** and create a thin mapping in the SQLite adapter (step 0.2). Alternatively, replace all `db.Game` references with `store.Game` project-wide. The latter is cleaner long-term.

**If replacing project-wide:** Change all imports in `game/*.go`, `bot/handlers/*.go`, and `bot/bot.go` from `db.Game` → `store.Game`, `db.Player` → `store.Player`, etc. Then delete the struct definitions from `db/` files (keep the functions).

### 0.2 — Create `store/store.go` (Store interface)

This interface must cover **every** `db.*` function currently called across the codebase. Here is the complete inventory:

```go
package store

// Store is the data access interface for all game state.
type Store interface {
    // Games
    CreateGame(joinCode, guildID, channelID, createdBy string) (int64, error)
    GetGameByID(gameID int64) (*Game, error)
    GetGameByJoinCode(joinCode string) (*Game, error)
    GetGameByChannel(channelID string) (*Game, error)
    GetFinishedGameByChannel(channelID string) (*Game, error)
    FinishAllGames(guildID string) (int64, error)
    UpdateGameStatus(gameID int64, status, phase string) error
    UpdateGameRound(gameID int64, round int) error
    UpdateGamePhase(gameID int64, phase string) error
    UpdateGameTimers(gameID int64, breakfast, roundtable, night, mission int) error
    UpdateGameBuyin(gameID int64, amountCents int) error
    UpdateGameEndBy(gameID int64, endBy string) error
    UpdateGameHiatus(gameID int64, start, end, tz string) error
    SetRecruitmentPending(gameID int64, pending bool) error
    SetTraitorThreadID(gameID int64, threadID string) error

    // Players
    AddPlayer(gameID int64, discordID, discordName string) error
    GetPlayer(gameID int64, discordID string) (*Player, error)
    GetAlivePlayers(gameID int64) ([]Player, error)
    GetAllPlayers(gameID int64) ([]Player, error)
    GetPlayersByRole(gameID int64, role string) ([]Player, error)
    GetPlayersByStatusAndRound(gameID int64, status string, round int) ([]Player, error)
    CountPlayersByStatus(gameID int64, status string) (int, error)
    UpdatePlayerRole(gameID int64, discordID, role string) error
    UpdatePlayerStatus(gameID int64, discordID, status string) error
    UpdatePlayerStatusWithRound(gameID int64, discordID, status string, round int) error
    UpdatePlayerShield(gameID int64, discordID string, hasShield bool) error
    UpdatePlayerWallet(gameID int64, discordID, walletInfo string) error
    SetPlayerRecruitedRound(gameID int64, discordID string, round int) error

    // Votes
    CastVote(gameID int64, round int, phase, voterID, targetID string) error
    GetVotes(gameID int64, round int, phase string) ([]Vote, error)
    CountVotes(gameID int64, round int, phase string) (int, error)
    ClearVotes(gameID int64, round int, phase string) error

    // Competitions
    CreateCompetition(gameID int64, round int, compType, questionData, answer string) (int64, error)
    GetActiveCompetition(gameID int64) (*Competition, error)
    SubmitCompetitionResult(competitionID int64, playerDiscordID, answer string, correct bool, timeMs int64) error
    GetCompetitionResults(competitionID int64) ([]CompetitionResult, error)
    EndCompetition(competitionID int64) error

    // Shields
    GrantShield(gameID int64, playerDiscordID, source string, round int) error
    ConsumeShield(gameID int64, playerDiscordID string, round int) error
    GetShieldLog(gameID int64) ([]ShieldLogEntry, error)

    // Payments
    MarkPaid(gameID int64, winnerDiscordID, loserDiscordID string) error
    IsMarkedPaid(gameID int64, winnerDiscordID, loserDiscordID string) (bool, error)
    GetPaymentsByWinner(gameID int64, winnerDiscordID string) ([]Payment, error)
    GetPaymentsByLoser(gameID int64, loserDiscordID string) ([]Payment, error)
}
```

### 0.3 — Create `store/sqlite/sqlite.go` (SQLite adapter)

Thin adapter wrapping existing `db.*` functions:

```go
package sqlite

import (
    "database/sql"
    "github.com/gatorjuice/async_traitors/db"
    "github.com/gatorjuice/async_traitors/store"
)

type SQLiteStore struct {
    db *sql.DB
}

func New(database *sql.DB) *SQLiteStore {
    return &SQLiteStore{db: database}
}

// --- Games ---

func (s *SQLiteStore) CreateGame(joinCode, guildID, channelID, createdBy string) (int64, error) {
    return db.CreateGame(s.db, joinCode, guildID, channelID, createdBy)
}

func (s *SQLiteStore) GetGameByID(gameID int64) (*store.Game, error) {
    g, err := db.GetGameByID(s.db, gameID)
    if err != nil {
        return nil, err
    }
    return convertGame(g), nil
}

// ... (one method per Store interface method, delegating to db.*)

// convertGame maps db.Game → store.Game (if you keep separate structs).
// If you made db.Game an alias for store.Game, these converters are unnecessary.
func convertGame(g *db.Game) *store.Game { /* field-by-field copy */ }
func convertPlayer(p *db.Player) *store.Player { /* field-by-field copy */ }
// etc.
```

**Simpler alternative:** If you replaced `db.Game` with `store.Game` in step 0.1, the `db.*` functions already return `*store.Game` and no conversion is needed. The adapter just delegates directly:

```go
func (s *SQLiteStore) GetGameByID(gameID int64) (*store.Game, error) {
    return db.GetGameByID(s.db, gameID)
}
```

### 0.4 — Create `game/scheduler.go` (TimerScheduler interface)

```go
package game

import "time"

// TimerScheduler manages per-game timers.
type TimerScheduler interface {
    StartTimer(gameID int64, duration time.Duration, onExpire func())
    ScheduleCallback(gameID int64, delay time.Duration, callback func())
    CancelTimer(gameID int64)
    HasActiveTimer(gameID int64) bool
}
```

`TimerManager` already satisfies this interface — its methods have matching signatures. No changes needed to `timer.go`.

### 0.5 — Refactor `game.Engine`

**Current** (`game/engine.go:16-20`):
```go
type Engine struct {
    DB      *sql.DB
    Session *discordgo.Session
    Timers  *TimerManager
}
```

**New:**
```go
type Engine struct {
    Store   store.Store
    Session *discordgo.Session
    Timers  TimerScheduler
}

func NewEngine(s store.Store, session *discordgo.Session) *Engine {
    return &Engine{
        Store:   s,
        Session: session,
        Timers:  NewTimerManager(),
    }
}
```

### 0.6 — Update all `db.*` calls in `game/*.go`

This is the bulk of the work. Every `db.FunctionName(e.DB, ...)` or `db.FunctionName(database, ...)` call must become `e.Store.FunctionName(...)` or `s.FunctionName(...)`.

Here is the **complete list of changes by file**:

#### `game/engine.go`

| Line(s) | Current | New |
|---|---|---|
| 33 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 42 | `db.GetAllPlayers(e.DB, gameID)` | `e.Store.GetAllPlayers(gameID)` |
| 67 | `db.UpdateGameTimers(e.DB, gameID, ...)` | `e.Store.UpdateGameTimers(gameID, ...)` |
| 80 | `db.SetTraitorThreadID(e.DB, gameID, thread.ID)` | `e.Store.SetTraitorThreadID(gameID, thread.ID)` |
| 84 | `db.GetPlayersByRole(e.DB, gameID, "traitor")` | `e.Store.GetPlayersByRole(gameID, "traitor")` |
| 95 | `db.UpdateGameStatus(e.DB, gameID, ...)` | `e.Store.UpdateGameStatus(gameID, ...)` |
| 98 | `db.UpdateGameRound(e.DB, gameID, 1)` | `e.Store.UpdateGameRound(gameID, 1)` |
| 118 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 131 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 195 | `db.UpdateGameRound(e.DB, gameID, newRound)` | `e.Store.UpdateGameRound(gameID, newRound)` |
| 207 | `db.UpdateGamePhase(e.DB, gameID, ...)` | `e.Store.UpdateGamePhase(gameID, ...)` |
| 212 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 233 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 238 | `db.GetPlayersByRole(e.DB, gameID, "traitor")` | `e.Store.GetPlayersByRole(gameID, "traitor")` |
| 243 | `db.GetPlayersByRole(e.DB, gameID, "faithful")` | `e.Store.GetPlayersByRole(gameID, "faithful")` |
| 262 | `db.GetPlayer(e.DB, gameID, banishedID)` | `e.Store.GetPlayer(gameID, banishedID)` |
| 267 | `db.GetPlayersByRole(e.DB, gameID, "traitor")` | `e.Store.GetPlayersByRole(gameID, "traitor")` |
| 272 | `db.GetAlivePlayers(e.DB, gameID)` | `e.Store.GetAlivePlayers(gameID)` |
| 277 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 284 | `db.SetRecruitmentPending(e.DB, gameID, true)` | `e.Store.SetRecruitmentPending(gameID, true)` |
| 316 | `db.GetPlayersByStatusAndRound(e.DB, game.ID, "murdered", prevRound)` | `e.Store.GetPlayersByStatusAndRound(game.ID, "murdered", prevRound)` |
| 322 | `db.GetShieldLog(e.DB, game.ID)` | `e.Store.GetShieldLog(game.ID)` |
| 407 | `db.GetAlivePlayers(e.DB, game.ID)` | `e.Store.GetAlivePlayers(game.ID)` |
| 426 | `db.GetVotes(e.DB, game.ID, game.CurrentRound, "roundtable")` | `e.Store.GetVotes(game.ID, game.CurrentRound, "roundtable")` |
| 451 | `db.GetGameByID(e.DB, game.ID)` | `e.Store.GetGameByID(game.ID)` |
| 469 | `db.GetAlivePlayers(e.DB, game.ID)` | `e.Store.GetAlivePlayers(game.ID)` |
| 496 | `db.GetAlivePlayers(e.DB, game.ID)` | `e.Store.GetAlivePlayers(game.ID)` |
| 512 | `db.GetPlayersByRole(e.DB, game.ID, "traitor")` | `e.Store.GetPlayersByRole(game.ID, "traitor")` |
| 516 | `db.GetVotes(e.DB, game.ID, game.CurrentRound, "night")` | `e.Store.GetVotes(game.ID, game.CurrentRound, "night")` |
| 591 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 596 | `db.GetPlayersByStatusAndRound(e.DB, gameID, "banished", completedRound)` | `e.Store.GetPlayersByStatusAndRound(gameID, "banished", completedRound)` |
| 600 | `db.GetPlayersByStatusAndRound(e.DB, gameID, "murdered", completedRound)` | `e.Store.GetPlayersByStatusAndRound(gameID, "murdered", completedRound)` |
| 604 | `db.GetAlivePlayers(e.DB, gameID)` | `e.Store.GetAlivePlayers(gameID)` |
| 608 | `db.GetShieldLog(e.DB, gameID)` | `e.Store.GetShieldLog(gameID)` |
| 676 | `db.GetGameByID(e.DB, gameID)` | `e.Store.GetGameByID(gameID)` |
| 681 | `db.UpdateGameStatus(e.DB, gameID, ...)` | `e.Store.UpdateGameStatus(gameID, ...)` |
| 698 | `db.GetAlivePlayers(e.DB, gameID)` | `e.Store.GetAlivePlayers(gameID)` |
| 712 | `db.GetAllPlayers(e.DB, gameID)` | `e.Store.GetAllPlayers(gameID)` |

Also remove the `"database/sql"` import and the `"github.com/gatorjuice/async_traitors/db"` import (add `"github.com/gatorjuice/async_traitors/store"` if model types moved there).

Change all `*db.Game` → `*store.Game` in method signatures (e.g., `startBreakfastPhase(game *store.Game)`).

#### `game/voting.go`

Functions `CastBanishmentVote` and `TallyBanishmentVotes` currently take `database *sql.DB` as first arg.

**Change signatures** to take `st store.Store`:

```go
func CastBanishmentVote(st store.Store, s *discordgo.Session, gameID int64, round int, voterID, targetID string) (bool, error)
func TallyBanishmentVotes(st store.Store, s *discordgo.Session, gameID int64, round int) (string, error)
```

Then replace all `db.X(database, ...)` → `st.X(...)`:

| Function | `db.*` calls to replace |
|---|---|
| `CastBanishmentVote` | `db.GetGameByID`, `db.GetPlayer` (×2), `db.CastVote`, `db.CountVotes`, `db.GetAlivePlayers` |
| `TallyBanishmentVotes` | `db.GetGameByID`, `db.GetVotes`, `db.GetPlayer` (multiple), `db.UpdatePlayerStatusWithRound`, `db.GetPlayer` (banished) |

Also update the `RevealRole` call at the bottom of `TallyBanishmentVotes` to pass `st` instead of `database`.

#### `game/night.go`

Same pattern — change `database *sql.DB` → `st store.Store`:

```go
func CastMurderVote(st store.Store, s *discordgo.Session, ...) (bool, error)
func ResolveNight(st store.Store, s *discordgo.Session, ...) error
```

| Function | `db.*` calls |
|---|---|
| `CastMurderVote` | `db.GetGameByID`, `db.GetPlayer` (×2), `db.CastVote`, `db.CountVotes`, `db.GetPlayersByRole` |
| `ResolveNight` | `db.GetGameByID`, `db.GetVotes`, `db.GetPlayer`, `db.ConsumeShield`, `db.UpdatePlayerStatusWithRound` |

#### `game/recruit.go`

Change `database *sql.DB` → `st store.Store` for `RecruitPlayer` and `ResolveRecruitment`.

For `AcceptRecruitment`, `RefuseRecruitment`, `ForceRecruit` — these take `e *Engine`. Change `e.DB` → `e.Store`:

| Function | `db.*` calls |
|---|---|
| `RecruitPlayer` | `db.GetGameByID`, `db.GetPlayer` (×2), `db.CastVote`, `db.CountVotes`, `db.GetPlayersByRole` |
| `ResolveRecruitment` | `db.GetGameByID`, `db.GetVotes`, `db.SetRecruitmentPending`, `db.GetPlayer` |
| `AcceptRecruitment` | `db.GetGameByID(e.DB,...)` (×1), `db.GetPlayer(e.DB,...)` (×1), `db.GetVotes(e.DB,...)`, `db.UpdatePlayerRole(e.DB,...)`, `db.SetPlayerRecruitedRound(e.DB,...)`, `db.GetPlayersByRole(e.DB,...)`, `db.SetRecruitmentPending(e.DB,...)` |
| `RefuseRecruitment` | `db.GetGameByID(e.DB,...)`, `db.GetPlayer(e.DB,...)`, `db.GetVotes(e.DB,...)`, `db.UpdatePlayerStatusWithRound(e.DB,...)`, `db.SetRecruitmentPending(e.DB,...)` |
| `ForceRecruit` | `db.GetGameByID(e.DB,...)`, `db.GetPlayer(e.DB,...)`, `db.UpdatePlayerRole(e.DB,...)`, `db.SetPlayerRecruitedRound(e.DB,...)`, `db.GetPlayersByRole(e.DB,...)`, `db.SetRecruitmentPending(e.DB,...)` |

#### `game/roles.go`

Change `database *sql.DB` → `st store.Store`:

```go
func AssignRoles(st store.Store, s *discordgo.Session, gameID int64) error
```

Calls: `db.GetAllPlayers`, `db.UpdatePlayerRole` (per player).

#### `game/shields.go`

Change `database *sql.DB` → `st store.Store` for all functions:

```go
func GrantShield(st store.Store, s *discordgo.Session, gameID int64, playerID, source string, round int) error
func ClaimShield(st store.Store, s *discordgo.Session, gameID int64, playerID string, round int) error
func ConsumeShield(st store.Store, s *discordgo.Session, gameID int64, playerID string, round int) error
func AdminGrantShield(st store.Store, s *discordgo.Session, gameID int64, playerID string, round int) error
```

Calls: `db.GrantShield`, `db.GetGameByID`, `db.GetPlayer`, `db.ConsumeShield`.

#### `game/reveal.go`

```go
func RevealRole(st store.Store, s *discordgo.Session, gameID int64, playerID string) error
```

Calls: `db.GetGameByID`, `db.GetPlayer`, `db.GetAlivePlayers`.

#### `game/buyin.go`

```go
func DetermineWinningSide(st store.Store, gameID int64) string
func SendPayoutDMs(s *discordgo.Session, st store.Store, gameID int64, winnerDiscordID string)
```

Calls: `db.GetPlayersByRole`, `db.GetGameByID`, `db.GetAllPlayers`, `db.GetPlayer`.

Note: `CalculatePayouts` takes `[]db.Player` — change to `[]store.Player`.

### 0.7 — Update `bot/bot.go`

**Current:**
```go
type Bot struct {
    Session *discordgo.Session
    DB      *sql.DB
    Config  *config.Config
    Engine  *game.Engine
}
```

**New:**
```go
type Bot struct {
    Session *discordgo.Session
    Store   store.Store
    Config  *config.Config
    Engine  *game.Engine
}
```

Change `New()`:
```go
func New(cfg *config.Config, st store.Store) (*Bot, error) {
    session, err := discordgo.New("Bot " + cfg.DiscordToken)
    // ...
    engine := game.NewEngine(st, session)
    return &Bot{Session: session, Store: st, Config: cfg, Engine: engine}, nil
}
```

Change all `b.DB` → `b.Store` in `handleInteraction` (passed to handlers).

Also update `detectActiveGames()` — this currently does a raw SQL query. Move it to the Store interface or remove it (it's a non-critical warning log).

### 0.8 — Update `bot/handlers/*.go`

Every handler currently takes `database *sql.DB`. Change to `st store.Store`:

```go
// Before:
func HandleCreateGame(s *discordgo.Session, i *discordgo.InteractionCreate, database *sql.DB)
// After:
func HandleCreateGame(s *discordgo.Session, i *discordgo.InteractionCreate, st store.Store)
```

Then replace all `db.X(database, ...)` → `st.X(...)` in each handler.

**Files and their `db.*` calls:**

| File | Functions using `db.*` |
|---|---|
| `handlers/admin.go` | `HandleCreateGame`: `db.CreateGame`, `db.AddPlayer`, `db.UpdateGameBuyin`, `db.UpdateGameEndBy`. `HandleSetTimers`: `db.GetGameByChannel`, `db.UpdateGameTimers`. `HandleSetBuyin`: `db.GetGameByChannel`, `db.UpdateGameBuyin`. `HandleSetHiatus`: `db.GetGameByChannel`, `db.UpdateGameHiatus`. `HandleEndGame`: `db.GetGameByChannel`, `db.UpdateGameStatus`. `HandleNukeGames`: `db.FinishAllGames`. |
| `handlers/player.go` | `HandleJoinGame`: `db.GetGameByJoinCode`, `db.AddPlayer`. `HandleJoinButton`: `db.GetGameByJoinCode`, `db.AddPlayer`. `HandleMyRole`: `db.GetGameByChannel`, `db.GetPlayer`, `db.GetPlayersByRole`. `HandleMarkPaid`: `db.GetFinishedGameByChannel`, `db.GetPlayer` (×2), `db.MarkPaid`, `db.GetAllPlayers`. `HandlePaymentStatus`: `db.GetFinishedGameByChannel`, `db.GetPlayer`, `db.GetAllPlayers`, `db.IsMarkedPaid` (loop). `HandleWallet`: `db.GetFinishedGameByChannel`, `db.GetPlayer`, `db.UpdatePlayerWallet`. |
| `handlers/game.go` | `HandleGameInfo`: `db.GetGameByChannel`, `db.CountPlayersByStatus`, `db.GetAllPlayers`. `HandlePlayers`: `db.GetGameByChannel`, `db.GetAllPlayers`. `HandleHelp`: none. `HandleRecap`: `db.GetShieldLog`, `db.GetPlayersByStatusAndRound` (loop), `db.GetAlivePlayers`. `HandleRules`: none. |
| `handlers/competition.go` | `HandleStartCompetition`: `db.GetGameByChannel`, `db.CreateCompetition`. `HandleSubmitAnswer`: `db.GetActiveCompetition`, `db.SubmitCompetitionResult`, `db.GetCompetitionResults`. `HandleEndCompetition`: `db.GetGameByChannel`, `db.GetActiveCompetition`, `db.GetCompetitionResults`, `db.EndCompetition`, `db.GetAlivePlayers`. |
| `handlers/stubs.go` | All dispatch to `game.*` functions — `HandleStartGame`: `db.GetGameByChannel`. `HandleVote`: `db.GetGameByChannel`. `HandleMurderVote`: `db.GetGameByChannel`. `HandleClaimShield`: none (uses `requirePlayer`). `HandleGrantShield`: `db.GetGameByChannel`. `HandleRecruit`: `db.GetGameByChannel`. `HandleAcceptRecruitment`: `db.GetGameByChannel`. `HandleRefuseRecruitment`: `db.GetGameByChannel`. `HandleForceRecruit`: `db.GetGameByChannel`. `HandleAdvancePhase`: `db.GetGameByChannel`. |
| `handlers/helpers.go` | `requirePlayer`: `db.GetGameByChannel`, `db.GetPlayer`. |

Also update `requirePlayer` helper:
```go
func requirePlayer(s *discordgo.Session, i *discordgo.InteractionCreate, st store.Store) (*store.Game, *store.Player) {
```

### 0.9 — Update `main.go`

```go
func main() {
    cfg, err := config.Load()
    // ...
    database, err := db.Open(cfg.DatabasePath)
    // ...
    defer database.Close()

    st := sqlite.New(database) // new

    b, err := bot.New(cfg, st) // was: bot.New(cfg, database)
    // ...
}
```

Add import: `"github.com/gatorjuice/async_traitors/store/sqlite"`.

### 0.10 — Update tests

Tests in `game/*_test.go` currently use `testDB()` which returns `*sql.DB`. They pass it directly to functions like `CastBanishmentVote(d, nil, ...)`.

Update `testhelper_test.go`:
```go
func testStore(t *testing.T) store.Store {
    t.Helper()
    d := testDB(t) // keep for raw SQL setup
    return sqlite.New(d)
}
```

Then update each test file to use `testStore(t)` where functions now expect `store.Store`, and `testDB(t)` where raw `*sql.DB` is still needed for setup helpers like `createTestGame`.

Alternatively, have `createTestGame` also accept `store.Store` and call `st.CreateGame(...)` + `st.AddPlayer(...)`.

---

## Phase 1: Discord HTTP Interactions Endpoint

**Goal:** Create a Lambda handler that receives Discord interaction POSTs via API Gateway (replacing the WebSocket gateway).

### 1.1 — Ed25519 signature verification

Create `discord/verify.go`:

```go
package discord

import (
    "crypto/ed25519"
    "encoding/hex"
    "errors"
)

// VerifySignature verifies a Discord interaction request signature.
// publicKey is the hex-encoded public key from Discord Developer Portal.
// signature is the X-Signature-Ed25519 header value.
// timestamp is the X-Signature-Timestamp header value.
// body is the raw request body.
func VerifySignature(publicKey, signature, timestamp string, body []byte) error {
    key, err := hex.DecodeString(publicKey)
    if err != nil {
        return errors.New("invalid public key")
    }

    sig, err := hex.DecodeString(signature)
    if err != nil {
        return errors.New("invalid signature")
    }

    msg := append([]byte(timestamp), body...)
    if !ed25519.Verify(key, msg, sig) {
        return errors.New("invalid signature")
    }
    return nil
}
```

### 1.2 — Deferred response pattern

Create `discord/respond.go`:

Discord requires a response within 3 seconds. Lambda cold starts can exceed this. The solution: immediately ACK with a deferred response (type 5), then follow up via REST webhook.

```go
package discord

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

const discordAPI = "https://discord.com/api/v10"

// DeferredResponse returns the JSON for a deferred channel message response.
func DeferredResponse() []byte {
    resp := map[string]interface{}{
        "type": 5, // DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE
    }
    b, _ := json.Marshal(resp)
    return b
}

// DeferredEphemeralResponse returns deferred + ephemeral.
func DeferredEphemeralResponse() []byte {
    resp := map[string]interface{}{
        "type": 5,
        "data": map[string]interface{}{
            "flags": 64, // EPHEMERAL
        },
    }
    b, _ := json.Marshal(resp)
    return b
}

// PingResponse returns the JSON for a PING acknowledgment.
func PingResponse() []byte {
    b, _ := json.Marshal(map[string]int{"type": 1})
    return b
}

// FollowUp sends a follow-up message via webhook.
func FollowUp(appID, interactionToken, content string) error {
    url := fmt.Sprintf("%s/webhooks/%s/%s", discordAPI, appID, interactionToken)
    body, _ := json.Marshal(map[string]string{"content": content})
    resp, err := http.Post(url, "application/json", bytes.NewReader(body))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("discord follow-up returned %d", resp.StatusCode)
    }
    return nil
}
```

### 1.3 — Lambda entry point

Create `cmd/lambda/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "log/slog"
    "os"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    "github.com/bwmarrin/discordgo"
    "github.com/gatorjuice/async_traitors/discord"
    // ... other imports
)

var (
    publicKey = os.Getenv("DISCORD_PUBLIC_KEY")
    appID     = os.Getenv("DISCORD_APP_ID")
    token     = os.Getenv("DISCORD_TOKEN")
    guildID   = os.Getenv("GUILD_ID")
    tableName = os.Getenv("TABLE_NAME")
)

func handler(ctx context.Context, event json.RawMessage) (interface{}, error) {
    // Determine event source: API Gateway or EventBridge
    var apiEvent events.APIGatewayV2HTTPRequest
    if err := json.Unmarshal(event, &apiEvent); err == nil && apiEvent.RequestContext.HTTP.Method != "" {
        return handleHTTP(ctx, apiEvent)
    }

    // EventBridge timer event
    var timerEvent TimerEvent
    if err := json.Unmarshal(event, &timerEvent); err == nil && timerEvent.Source == "timer" {
        return handleTimer(ctx, timerEvent)
    }

    return nil, fmt.Errorf("unknown event type")
}

func handleHTTP(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    // Verify signature
    sig := req.Headers["x-signature-ed25519"]
    ts := req.Headers["x-signature-timestamp"]
    if err := discord.VerifySignature(publicKey, sig, ts, []byte(req.Body)); err != nil {
        return events.APIGatewayV2HTTPResponse{StatusCode: 401}, nil
    }

    // Parse interaction type
    var interaction struct {
        Type int `json:"type"`
    }
    json.Unmarshal([]byte(req.Body), &interaction)

    // Handle PING
    if interaction.Type == 1 {
        return events.APIGatewayV2HTTPResponse{
            StatusCode: 200,
            Body:       string(discord.PingResponse()),
            Headers:    map[string]string{"Content-Type": "application/json"},
        }, nil
    }

    // For all other interactions: ACK immediately, process async
    // Parse full interaction
    var ic discordgo.InteractionCreate
    json.Unmarshal([]byte(req.Body), &ic)

    // Create session (REST-only, no gateway)
    session, _ := discordgo.New("Bot " + token)

    // Create store (DynamoDB)
    st := dynamo.New(tableName)

    // Create engine
    eng := game.NewEngine(st, session)
    // Set engine timers to EventBridge scheduler
    eng.Timers = eventbridge.NewScheduler(tableName)

    // Route and handle
    go func() {
        discord.RouteInteraction(session, &ic, st, eng)
    }()

    // Return deferred response
    return events.APIGatewayV2HTTPResponse{
        StatusCode: 200,
        Body:       string(discord.DeferredEphemeralResponse()),
        Headers:    map[string]string{"Content-Type": "application/json"},
    }, nil
}

func main() {
    lambda.Start(handler)
}
```

**Important:** The `go func()` pattern above is simplified. In Lambda, the function may terminate after returning the response. A better approach is to use the deferred response synchronously: return the ACK immediately as the HTTP response, then do the work in the same goroutine before the Lambda context expires. Or use Lambda response streaming.

The actual implementation should:
1. Return the deferred ACK as the HTTP response
2. Process the interaction in the same invocation (Lambda gives 30s timeout)
3. Send the result via webhook follow-up

### 1.4 — Interaction router

Create `discord/router.go`:

```go
package discord

import (
    "github.com/bwmarrin/discordgo"
    "github.com/gatorjuice/async_traitors/bot/handlers"
    "github.com/gatorjuice/async_traitors/game"
    "github.com/gatorjuice/async_traitors/store"
)

// RouteInteraction dispatches an interaction to the appropriate handler.
// Mirrors bot/bot.go handleInteraction but works without gateway.
func RouteInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, st store.Store, eng *game.Engine) {
    switch i.Type {
    case discordgo.InteractionMessageComponent:
        // handle join-game button etc.
    case discordgo.InteractionApplicationCommand:
        name := i.ApplicationCommandData().Name
        switch name {
        case "create-game":
            handlers.HandleCreateGame(s, i, st)
        case "start-game":
            handlers.HandleStartGame(s, i, st, eng)
        // ... all other commands, same as bot/bot.go:handleInteraction
        }
    }
}
```

### 1.5 — New config vars

Add to `config/config.go`:

```go
type Config struct {
    DiscordToken     string
    DiscordPublicKey string // DISCORD_PUBLIC_KEY
    DiscordAppID     string // DISCORD_APP_ID
    GuildID          string
    DatabasePath     string
    TableName        string // TABLE_NAME (DynamoDB, Lambda only)
}
```

### New dependencies

```bash
go get github.com/aws/aws-lambda-go
go get github.com/aws/aws-sdk-go-v2/...
```

---

## Phase 2: DynamoDB Store Implementation

**Goal:** Implement `store.Store` for DynamoDB using single-table design.

### Key schema (single table)

| Entity | PK | SK | GSI1-PK | GSI1-SK |
|---|---|---|---|---|
| Game | `GAME#<id>` | `META` | `CHANNEL#<channelID>` | `STATUS#<status>` |
| Game (join code lookup) | `GAME#<id>` | `META` | `JOINCODE#<code>` | `GAME` |
| Player | `GAME#<id>` | `PLAYER#<discordID>` | | |
| Vote | `GAME#<id>` | `VOTE#<round>#<phase>#<voterID>` | | |
| Competition | `GAME#<id>` | `COMP#<round>` | | |
| Comp Result | `GAME#<id>` | `COMPRESULT#<compID>#<playerID>` | | |
| Shield Log | `GAME#<id>` | `SHIELD#<playerID>#<roundGranted>` | | |
| Payment | `GAME#<id>` | `PAYMENT#<winnerID>#<loserID>` | | |
| Timer | `GAME#<id>` | `TIMER#<type>` | | |
| Counter | `SYSTEM` | `COUNTER#GAME` | | |

**Game ID generation:** Atomic counter using `UpdateItem` with `ADD` on `SYSTEM / COUNTER#GAME`.

**GSI1 note:** The game needs to be queryable by both `channelID` (for `GetGameByChannel`) and `joinCode` (for `GetGameByJoinCode`). Two options:
- Two GSI entries per game (write the game item twice with different GSI1-PK values) — complex
- Use two separate GSI items: the main game item has `GSI1-PK=CHANNEL#<ch>`, and a separate "join code index" item with `GSI1-PK=JOINCODE#<code>` pointing to the game ID — simpler

### Implementation files

#### `store/dynamo/dynamo.go`

```go
package dynamo

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type DynamoStore struct {
    client    *dynamodb.Client
    tableName string
}

func New(tableName string) (*DynamoStore, error) {
    cfg, err := config.LoadDefaultConfig(context.TODO())
    if err != nil {
        return nil, err
    }
    return &DynamoStore{
        client:    dynamodb.NewFromConfig(cfg),
        tableName: tableName,
    }, nil
}

// nextGameID atomically increments the game counter.
func (d *DynamoStore) nextGameID() (int64, error) {
    // UpdateItem on PK=SYSTEM, SK=COUNTER#GAME
    // SET counter = counter + 1
    // ReturnValues: UPDATED_NEW
}
```

#### `store/dynamo/games.go`

Implement all game methods. Key patterns:

- `CreateGame` → `nextGameID()` + `PutItem` + join code GSI entry
- `GetGameByID` → `GetItem` with `PK=GAME#<id>, SK=META`
- `GetGameByChannel` → `Query GSI1` where `GSI1-PK=CHANNEL#<channelID>` and `GSI1-SK begins_with STATUS#` (filter out "finished")
- `GetGameByJoinCode` → `Query GSI1` where `GSI1-PK=JOINCODE#<code>`
- `GetFinishedGameByChannel` → `Query GSI1` where `GSI1-PK=CHANNEL#<ch>` and `GSI1-SK=STATUS#finished`
- `UpdateGameStatus` → `UpdateItem` — **also update GSI1-SK** to reflect new status
- `FinishAllGames` → `Query` all non-finished games by guild, batch update (DynamoDB doesn't have SQL-style WHERE clauses)

**Important:** Use `ConsistentRead: true` for reads that feed into "all voted?" checks.

#### `store/dynamo/players.go`

- `AddPlayer` → `PutItem` with `PK=GAME#<id>, SK=PLAYER#<discordID>`
- `GetPlayer` → `GetItem`
- `GetAlivePlayers` → `Query PK=GAME#<id>` with `begins_with(SK, "PLAYER#")` + `FilterExpression: #status = :alive`
- `GetPlayersByRole` → Same query + filter on role AND status=alive
- `GetPlayersByStatusAndRound` → Same query + filter on status AND status_round

#### `store/dynamo/votes.go`

- `CastVote` → `PutItem` (SK includes voter ID, so upsert is natural)
- `GetVotes` → `Query PK=GAME#<id>` with `begins_with(SK, "VOTE#<round>#<phase>#")`
- `CountVotes` → Same query, use `Select: COUNT`
- `ClearVotes` → Query + `BatchWriteItem` delete

#### `store/dynamo/competitions.go`

- `CreateCompetition` → `PutItem` with `SK=COMP#<round>`
- `GetActiveCompetition` → `Query` with `begins_with(SK, "COMP#")` + filter `status=active`
- `SubmitCompetitionResult` → `PutItem` with `SK=COMPRESULT#<compID>#<playerID>`
- `GetCompetitionResults` → `Query` with `begins_with(SK, "COMPRESULT#<compID>#")`
- `EndCompetition` → `UpdateItem` set status=completed
- Shield operations: `GrantShield` → `PutItem` for shield log + `UpdateItem` player has_shield=true (use `TransactWriteItems`)
- `ConsumeShield` → `TransactWriteItems` (conditional on has_shield=true)
- `GetShieldLog` → `Query` with `begins_with(SK, "SHIELD#")`

#### `store/dynamo/payments.go`

- `MarkPaid` → `PutItem` with condition `attribute_not_exists(PK)` (idempotent)
- `IsMarkedPaid` → `GetItem`
- `GetPaymentsByWinner/Loser` → `Query` with `begins_with(SK, "PAYMENT#<id>#")` — may need both directions, or store both PK patterns

### Testing with DynamoDB Local

```bash
# docker-compose.yml for local dev
docker run -p 8000:8000 amazon/dynamodb-local

# Or use the official Docker image in CI
```

Create a test helper that stands up DynamoDB Local and creates the table for tests.

---

## Phase 3: EventBridge Timer System

**Goal:** Replace in-memory goroutine timers with EventBridge Scheduler for serverless execution.

### 3.1 — EventBridge TimerScheduler

Create `timer/eventbridge.go`:

```go
package timer

import (
    "context"
    "fmt"
    "time"

    "github.com/aws/aws-sdk-go-v2/service/scheduler"
    "github.com/gatorjuice/async_traitors/game"
)

type EventBridgeScheduler struct {
    client    *scheduler.Client
    tableName string
    lambdaARN string
    roleARN   string
}

func (eb *EventBridgeScheduler) StartTimer(gameID int64, duration time.Duration, onExpire func()) {
    // 1. Cancel any existing timers for this game
    eb.CancelTimer(gameID)

    // 2. Create EventBridge one-time schedule
    //    Name: "game-<gameID>-phase-advance"
    //    ScheduleExpression: "at(2026-04-05T12:00:00)"
    //    Target: Lambda ARN
    //    Input: {"source":"timer","gameID":<gameID>,"type":"phase_advance"}

    // 3. Store timer metadata in DynamoDB (GAME#<id>, TIMER#phase_advance)
    //    with scheduleName and expectedFireTime

    // Note: onExpire callback is NOT used directly — instead, when the
    // EventBridge schedule fires, it invokes Lambda which calls AdvancePhase.
}

func (eb *EventBridgeScheduler) ScheduleCallback(gameID int64, delay time.Duration, callback func()) {
    // Create additional schedules for warnings
    // Name: "game-<gameID>-warning-<hash>"
    // Input: {"source":"timer","gameID":<gameID>,"type":"warning","delay_ms":<delay>}
}

func (eb *EventBridgeScheduler) CancelTimer(gameID int64) {
    // 1. Query DynamoDB for all TIMER# entries for this game
    // 2. Delete each corresponding EventBridge schedule by name
    // 3. Delete the DynamoDB timer entries
}

func (eb *EventBridgeScheduler) HasActiveTimer(gameID int64) bool {
    // Query DynamoDB for TIMER# entries for this game
}
```

**Key difference from in-memory:** The `onExpire` callback is not used directly. Instead, EventBridge invokes Lambda with a timer event payload. The Lambda handler then calls `engine.AdvancePhase()`.

### 3.2 — Timer event handling in Lambda

Add to `cmd/lambda/main.go`:

```go
type TimerEvent struct {
    Source string `json:"source"` // "timer"
    GameID int64  `json:"gameID"`
    Type   string `json:"type"`   // "phase_advance", "warning_half", "warning_5min"
}

func handleTimer(ctx context.Context, event TimerEvent) error {
    st := dynamo.New(tableName)
    session, _ := discordgo.New("Bot " + token)
    eng := game.NewEngine(st, session)
    eng.Timers = eventbridge.NewScheduler(tableName)

    switch event.Type {
    case "phase_advance":
        // Check if game is still in expected phase (concurrency guard)
        g, err := st.GetGameByID(event.GameID)
        if err != nil || g.Status != "active" {
            return nil // Game ended or doesn't exist
        }

        // Handle hiatus
        if game.IsInHiatus(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now()) {
            // Reschedule for after hiatus
            wait := game.TimeUntilHiatusEnd(g.HiatusStart, g.HiatusEnd, g.HiatusTimezone, time.Now())
            eng.Timers.StartTimer(event.GameID, wait, nil)
            return nil
        }

        return eng.AdvancePhase(event.GameID)

    case "warning_half", "warning_5min":
        // Reconstruct warning message logic
        // Send appropriate DMs/channel messages
    }
    return nil
}
```

### 3.3 — Concurrency guard

Use DynamoDB conditional writes on phase transitions to prevent double-advancement:

```go
// In DynamoStore.UpdateGamePhase:
func (d *DynamoStore) UpdateGamePhaseConditional(gameID int64, expectedPhase, newPhase string) error {
    // UpdateItem with ConditionExpression: current_phase = :expected
    // If the condition fails, another process already advanced the phase
}
```

This prevents the race condition where a timer fires at the same moment the last player votes and both try to advance the phase.

---

## Phase 4: SAM Template & AWS Infrastructure

### 4.1 — `template.yaml`

```yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31
Description: Async Traitors Discord Bot (Lambda)

Parameters:
  DiscordToken:
    Type: String
    NoEcho: true
  DiscordPublicKey:
    Type: String
  DiscordAppID:
    Type: String
  GuildID:
    Type: String
  TableName:
    Type: String
    Default: async-traitors

Globals:
  Function:
    Timeout: 30
    MemorySize: 128
    Runtime: provided.al2023
    Architectures:
      - arm64
    Environment:
      Variables:
        DISCORD_TOKEN: !Ref DiscordToken
        DISCORD_PUBLIC_KEY: !Ref DiscordPublicKey
        DISCORD_APP_ID: !Ref DiscordAppID
        GUILD_ID: !Ref GuildID
        TABLE_NAME: !Ref TableName

Resources:
  HttpApi:
    Type: AWS::Serverless::HttpApi
    Properties:
      StageName: prod

  BotFunction:
    Type: AWS::Serverless::Function
    Properties:
      Handler: bootstrap
      CodeUri: .
      Events:
        Interactions:
          Type: HttpApi
          Properties:
            ApiId: !Ref HttpApi
            Path: /interactions
            Method: POST
        TimerEvent:
          Type: Schedule
          Properties:
            Schedule: rate(1 minute)  # placeholder — actual schedules created dynamically
            Enabled: false
      Policies:
        - DynamoDBCrudPolicy:
            TableName: !Ref GameTable
        - Statement:
            - Effect: Allow
              Action:
                - scheduler:CreateSchedule
                - scheduler:DeleteSchedule
                - scheduler:GetSchedule
              Resource: "*"
        - Statement:
            - Effect: Allow
              Action: iam:PassRole
              Resource: !GetAtt SchedulerRole.Arn

  GameTable:
    Type: AWS::DynamoDB::Table
    Properties:
      TableName: !Ref TableName
      BillingMode: PAY_PER_REQUEST
      AttributeDefinitions:
        - AttributeName: PK
          AttributeType: S
        - AttributeName: SK
          AttributeType: S
        - AttributeName: GSI1PK
          AttributeType: S
        - AttributeName: GSI1SK
          AttributeType: S
      KeySchema:
        - AttributeName: PK
          KeyType: HASH
        - AttributeName: SK
          KeyType: RANGE
      GlobalSecondaryIndexes:
        - IndexName: GSI1
          KeySchema:
            - AttributeName: GSI1PK
              KeyType: HASH
            - AttributeName: GSI1SK
              KeyType: RANGE
          Projection:
            ProjectionType: ALL

  SchedulerRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Version: '2012-10-17'
        Statement:
          - Effect: Allow
            Principal:
              Service: scheduler.amazonaws.com
            Action: sts:AssumeRole
      Policies:
        - PolicyName: InvokeLambda
          PolicyDocument:
            Version: '2012-10-17'
            Statement:
              - Effect: Allow
                Action: lambda:InvokeFunction
                Resource: !GetAtt BotFunction.Arn

  EventBridgePermission:
    Type: AWS::Lambda::Permission
    Properties:
      FunctionName: !Ref BotFunction
      Action: lambda:InvokeFunction
      Principal: scheduler.amazonaws.com
      SourceArn: !GetAtt SchedulerRole.Arn

Outputs:
  ApiUrl:
    Description: API Gateway endpoint URL (set this in Discord Developer Portal)
    Value: !Sub "https://${HttpApi}.execute-api.${AWS::Region}.amazonaws.com/prod/interactions"
  TableName:
    Value: !Ref GameTable
```

### 4.2 — `Makefile`

```makefile
.PHONY: build deploy register test

build:
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap cmd/lambda/main.go

deploy: build
	sam deploy --guided

register:
	go run cmd/register/main.go

test:
	go test ./...
```

### 4.3 — GitHub Actions OIDC Setup (manual AWS console steps)

1. In AWS IAM, create OIDC identity provider for `token.actions.githubusercontent.com`
2. Create IAM role `async-traitors-deploy` with trust policy:
   ```json
   {
     "Effect": "Allow",
     "Principal": {"Federated": "arn:aws:iam::<ACCOUNT>:oidc-provider/token.actions.githubusercontent.com"},
     "Action": "sts:AssumeRoleWithWebIdentity",
     "Condition": {
       "StringEquals": {"token.actions.githubusercontent.com:aud": "sts.amazonaws.com"},
       "StringLike": {"token.actions.githubusercontent.com:sub": "repo:gatorjuice/async_traitors:*"}
     }
   }
   ```
3. Attach policies: `AWSLambda_FullAccess`, `AmazonDynamoDBFullAccess`, `AmazonAPIGatewayAdministrator`, `IAMFullAccess`, `AmazonEventBridgeSchedulerFullAccess`, `AWSCloudFormationFullAccess`, `AmazonS3FullAccess`
4. Store role ARN as `AWS_DEPLOY_ROLE_ARN` in GitHub repo secrets

### 4.4 — GitHub Secrets

| Secret | Source |
|---|---|
| `AWS_DEPLOY_ROLE_ARN` | From step 4.3 |
| `DISCORD_PUBLIC_KEY` | Discord Developer Portal → General Information |
| `DISCORD_APP_ID` | Discord Developer Portal → General Information |
| `DISCORD_TOKEN` | Already exists |
| `GUILD_ID` | Already exists |

---

## Phase 5: Slash Command Registration

Create `cmd/register/main.go`:

```go
package main

import (
    "log"
    "os"

    "github.com/bwmarrin/discordgo"
    "github.com/gatorjuice/async_traitors/bot"
)

func main() {
    token := os.Getenv("DISCORD_TOKEN")
    guildID := os.Getenv("GUILD_ID")
    appID := os.Getenv("DISCORD_APP_ID")

    session, err := discordgo.New("Bot " + token)
    if err != nil {
        log.Fatal(err)
    }

    _, err = session.ApplicationCommandBulkOverwrite(appID, guildID, bot.Commands)
    if err != nil {
        log.Fatal("failed to register commands:", err)
    }

    log.Println("commands registered successfully")
}
```

This reuses the existing `bot.Commands` slice from `bot/commands.go`.

Run: `DISCORD_TOKEN=... GUILD_ID=... DISCORD_APP_ID=... go run cmd/register/main.go`

---

## Phase 6: CI/CD Pipeline

Create `.github/workflows/deploy-lambda.yml`:

```yaml
name: Deploy Lambda

on:
  push:
    branches: [main]

permissions:
  id-token: write
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run tests
        run: go test ./...

      - name: Build Lambda binary
        run: GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap cmd/lambda/main.go

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_DEPLOY_ROLE_ARN }}
          aws-region: us-east-1

      - uses: aws-actions/setup-sam@v2

      - name: SAM deploy
        run: |
          sam deploy --no-confirm-changeset --no-fail-on-empty-changeset \
            --parameter-overrides \
              DiscordToken=${{ secrets.DISCORD_TOKEN }} \
              DiscordPublicKey=${{ secrets.DISCORD_PUBLIC_KEY }} \
              DiscordAppID=${{ secrets.DISCORD_APP_ID }} \
              GuildID=${{ secrets.GUILD_ID }}

      - name: Register slash commands
        env:
          DISCORD_TOKEN: ${{ secrets.DISCORD_TOKEN }}
          GUILD_ID: ${{ secrets.GUILD_ID }}
          DISCORD_APP_ID: ${{ secrets.DISCORD_APP_ID }}
        run: go run cmd/register/main.go
```

Keep old `.github/workflows/deploy.yml` until EC2 is decommissioned.

---

## Phase 7: Data Migration & Cutover

### 7.1 — Migration script

Create `cmd/migrate/main.go`:

```go
package main

// Reads SQLite database, writes all data to DynamoDB.
// Usage: TABLE_NAME=async-traitors DATABASE_PATH=async_traitors.db go run cmd/migrate/main.go

func main() {
    // 1. Open SQLite
    // 2. Connect to DynamoDB
    // 3. For each game: write GAME#<id>/META item
    // 4. For each player: write GAME#<id>/PLAYER#<discordID> item
    // 5. For each vote: write GAME#<id>/VOTE#<round>#<phase>#<voterID> item
    // 6. For each competition: write GAME#<id>/COMP#<round> item
    // 7. For each competition_result: write GAME#<id>/COMPRESULT#<compID>#<playerID> item
    // 8. For each shield_log: write GAME#<id>/SHIELD#<playerID>#<roundGranted> item
    // 9. For each payment: write GAME#<id>/PAYMENT#<winnerID>#<loserID> item
    // 10. Set game ID counter to max(game.id)
}
```

### 7.2 — Cutover checklist

1. Ensure no active games: check with `/game-info` or query DB
2. Back up SQLite: `scp ec2-user@<EC2_HOST>:/opt/async-traitors-data/async_traitors.db ./backup.db`
3. Run migration: `TABLE_NAME=async-traitors DATABASE_PATH=backup.db go run cmd/migrate/main.go`
4. Deploy Lambda: `sam deploy` (or push to main for CI/CD)
5. Note the API Gateway URL from SAM outputs
6. **Set Discord Interactions Endpoint URL:**
   - Discord Developer Portal → Your Application → General Information
   - Set "Interactions Endpoint URL" to the API Gateway URL
   - Discord will send a PING to verify — Lambda must respond correctly
7. Register slash commands: `go run cmd/register/main.go`
8. **Test:** Create a game, join, start, play through a round
9. If successful, proceed to Phase 8

---

## Phase 8: EC2 Decommission

### 8.1 — Disable old CI/CD

- Delete `.github/workflows/deploy.yml` (or rename to `.deploy.yml.bak`)
- Remove GitHub Actions secrets: `EC2_HOST`, `EC2_SSH_KEY`

### 8.2 — Stop EC2 bot

```bash
ssh ec2-user@<EC2_HOST>
docker stop async-traitors
docker rm async-traitors
```

### 8.3 — Terminate AWS resources

1. Terminate EC2 instance
2. Delete EBS volume (if not auto-deleted)
3. Release Elastic IP
4. Delete Security Group
5. Delete SSH key pair (optional)

### 8.4 — Clean up repo

- Delete `Dockerfile`
- Delete old deploy workflow
- Update README

---

## File Inventory

### New files to create

| Phase | File | Description |
|---|---|---|
| 0 | `store/store.go` | Store interface |
| 0 | `store/models.go` | Shared model structs |
| 0 | `store/sqlite/sqlite.go` | SQLite Store adapter |
| 0 | `game/scheduler.go` | TimerScheduler interface |
| 1 | `discord/verify.go` | Ed25519 signature verification |
| 1 | `discord/respond.go` | Deferred response helpers |
| 1 | `discord/router.go` | Interaction router |
| 1 | `cmd/lambda/main.go` | Lambda entry point |
| 2 | `store/dynamo/dynamo.go` | DynamoDB Store constructor |
| 2 | `store/dynamo/games.go` | Game CRUD |
| 2 | `store/dynamo/players.go` | Player CRUD |
| 2 | `store/dynamo/votes.go` | Vote operations |
| 2 | `store/dynamo/competitions.go` | Competition + shield operations |
| 2 | `store/dynamo/payments.go` | Payment operations |
| 3 | `timer/eventbridge.go` | EventBridge TimerScheduler |
| 4 | `template.yaml` | SAM template |
| 4 | `Makefile` | Build targets |
| 5 | `cmd/register/main.go` | Slash command registration CLI |
| 6 | `.github/workflows/deploy-lambda.yml` | Lambda CI/CD pipeline |
| 7 | `cmd/migrate/main.go` | SQLite → DynamoDB migration |

### Existing files to modify

| Phase | File | Change |
|---|---|---|
| 0 | `game/engine.go` | `DB *sql.DB` → `Store store.Store`, `Timers *TimerManager` → `Timers TimerScheduler`, all `db.*` calls |
| 0 | `game/voting.go` | `database *sql.DB` → `st store.Store`, all `db.*` calls |
| 0 | `game/night.go` | Same |
| 0 | `game/recruit.go` | Same (+ `e.DB` → `e.Store` in Engine methods) |
| 0 | `game/roles.go` | Same |
| 0 | `game/shields.go` | Same |
| 0 | `game/reveal.go` | Same |
| 0 | `game/buyin.go` | Same |
| 0 | `bot/bot.go` | `DB *sql.DB` → `Store store.Store`, pass through to handlers |
| 0 | `bot/handlers/admin.go` | `database *sql.DB` → `st store.Store`, all `db.*` calls |
| 0 | `bot/handlers/player.go` | Same |
| 0 | `bot/handlers/game.go` | Same |
| 0 | `bot/handlers/competition.go` | Same |
| 0 | `bot/handlers/stubs.go` | Same |
| 0 | `bot/handlers/helpers.go` | Same |
| 0 | `main.go` | Create SQLite store adapter, pass to `bot.New()` |
| 0 | `game/*_test.go` | Use `store.Store` where functions changed |
| 1 | `config/config.go` | Add `DiscordPublicKey`, `DiscordAppID`, `TableName` |

### Files to delete (Phase 8)

| File | Reason |
|---|---|
| `Dockerfile` | No longer running on EC2 |
| `.github/workflows/deploy.yml` | Replaced by `deploy-lambda.yml` |

---

## Estimated Costs

| Service | Monthly Cost |
|---|---|
| Lambda | $0 (free tier: 1M requests) |
| API Gateway | $0 (free tier: 1M requests) |
| DynamoDB | $0 (free tier: 25 WCU/RCU) |
| EventBridge Scheduler | $0 (free tier: 14M invocations) |
| **Total** | **~$0/month** |

Current EC2 cost: ~$10/month.
