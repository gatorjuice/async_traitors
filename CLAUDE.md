# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Async Traitors is a Discord bot that runs asynchronous social deduction games inspired by "The Traitors". Games play out over a long weekend with async phases so players can participate at their own pace. Written in Go, using `discordgo` for the Discord API and pure-Go SQLite (`modernc.org/sqlite`, no CGo).

## Commands

```bash
# Run
go run .                          # Start bot (requires .env with DISCORD_TOKEN, GUILD_ID)

# Build
go build -o async_traitors .     # Build binary
CGO_ENABLED=0 go build -o async_traitors .  # Static binary (production)

# Test
go test ./...                     # All tests
go test -v ./game                 # Verbose, single package
go test -run TestTallyBanishment ./game  # Single test

# Lint/Format
go fmt ./...
go vet ./...

# Docker
docker build -t async_traitors .
docker run --env-file .env async_traitors
```

## Architecture

```
main.go → config.Load() → db.Open() → bot.New() → bot.Start()
```

**Layered design with strict separation:**

- **bot/** — Discord session lifecycle, slash command registration (17 commands in `commands.go`), interaction routing to handlers
- **bot/handlers/** — Translates Discord interactions into game engine calls. Handlers receive `*discordgo.Session`, `*discordgo.InteractionCreate`, and `*game.Engine`. Split by concern: `admin.go`, `player.go`, `game.go`, `competition.go`, `stubs.go`
- **game/** — Core game logic, completely decoupled from Discord. `Engine` struct holds `*sql.DB`, `*discordgo.Session`, and `*TimerManager`. Key files:
  - `engine.go` — Game loop, phase transitions, win condition checks
  - `state.go` — Phase/role/status enums and valid transition map
  - `voting.go` — Banishment votes (secret until all cast or timer expires)
  - `night.go` — Murder votes with early resolution when all traitors vote
  - `timer.go` — Per-game timers using `context.Context` for cancellation
  - `roles.go` — Crypto-random role assignment (Fisher-Yates shuffle)
  - `shields.go` — Shield grant/consume logic
- **db/** — SQLite with WAL mode. Schema in `migrations.go` (6 tables: games, players, votes, competitions, competition_results, shield_log). All queries parameterized.
- **competition/** — Registry pattern for extensible competition types. Each implements `Generate()`, `CheckAnswer()`, `Type()`, `Description()`. Types: trivia, speed, puzzle, scavenger.
- **notify/** — Discord message/embed helpers. Nil-safe so `Session` can be nil in unit tests.
- **config/** — Loads from `.env` via godotenv: `DISCORD_TOKEN`, `GUILD_ID`, `DATABASE_PATH`

## Game State Machine

**Game status:** `lobby → active → finished`

**Phase cycle per round:** `competition → discussion → voting → night → (next round)`

Phases auto-advance via timers or immediately when all eligible players act (all alive players vote, or all traitors submit murder votes).

## Testing Patterns

- Game logic tests use nil `discordgo.Session` since `notify/` is nil-safe
- `game/testhelper_test.go` provides shared test utilities
- DB tests use in-memory SQLite (`:memory:`)
- Tests exist for `game/`, `db/`, and `competition/` packages (~40 tests total)
- `bot/` and `bot/handlers/` are untested (integration layer)

## Configuration

Copy `.env.example` to `.env` and set `DISCORD_TOKEN` and `GUILD_ID`. The bot requires Discord permissions: Send Messages, Create Private Threads, Manage Threads, Send Messages in Threads, Use Slash Commands, Embed Links.
