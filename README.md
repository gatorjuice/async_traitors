# Async Traitors

A Discord bot that runs asynchronous social deduction games (inspired by TV's "The Traitors") played over a long weekend.

## Prerequisites

- Go 1.26+
- A Discord bot token with the following permissions: Send Messages, Create Private Threads, Manage Threads, Send Messages in Threads, Use Slash Commands, Embed Links

## Setup

```bash
git clone <repo>
cd async_traitors
cp .env.example .env
# Edit .env with your bot token and guild ID
go run .
```

## Docker

```bash
docker build -t async_traitors .
docker run --env-file .env async_traitors
```

## Game Flow

1. Admin creates a game with `/create-game` — gets a join code
2. Players join with `/join-game code:ABC123`
3. Admin starts with `/start-game` — roles assigned via DM
4. Each round cycles through phases:
   - **Competition** — admin starts with `/start-competition`, players answer with `/submit-answer`
   - **Discussion** — players discuss who they suspect
   - **Voting** — players `/vote` to banish someone (public votes)
   - **Night** — traitors `/murder-vote` in their private thread
5. Game ends when all traitors are banished (faithful win) or traitors outnumber faithful (traitors win)

## Commands

### Player Commands
| Command | Description |
|---------|-------------|
| `/join-game` | Join a game with a code |
| `/my-role` | Check your secret role (sent via DM) |
| `/vote` | Vote to banish a player |
| `/murder-vote` | Vote to murder a player (traitors only) |
| `/submit-answer` | Submit your competition answer |
| `/claim-shield` | Claim a shield (honor system) |
| `/game-info` | Show current game status |
| `/players` | List all players |
| `/help` | Show help and rules |

### Admin Commands
| Command | Description |
|---------|-------------|
| `/create-game` | Create a new game |
| `/start-game` | Start the game |
| `/start-competition` | Start a competition round |
| `/end-competition` | End current competition |
| `/grant-shield` | Grant a shield to a player |
| `/set-timers` | Configure phase timers |
| `/advance-phase` | Manually advance to next phase |
| `/end-game` | Force-end the game |

## Architecture

- **Go** with `discordgo` for Discord interaction
- **SQLite** (pure Go, no CGo) for persistence
- **Timer system** with `context.Context` cancellation for auto-advancing phases
- Game logic is separated from Discord calls for testability
