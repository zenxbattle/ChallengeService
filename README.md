# ZenXBattle Challenge Service

Real-time coding battle orchestrator. Manages challenge lifecycle (create, join, submit, judge), WebSocket connections for live updates, and leaderboard synchronization.

## Architecture

```
Browser ↔ WebSocket (wss) ← ChallengeService (:50052)
           gRPC calls → CodeExecutionEngine (code judge)
           gRPC calls → RedisBoard (leaderboard)
           PostgreSQL (challenge state)
```

## Tech Stack

- **Go** + gRPC
- **WebSockets** (gorilla/websocket) for real-time battle state
- **PostgreSQL** for challenge persistence  
- **Redis** for leaderboard
- **ants** goroutine pool for concurrent match handling
- **JWT** for player auth within challenges

## Challenge Lifecycle

```
Create → Wait → Start → Battle → Judge → Complete
   │        │       │       │       │        │
   │   players     │    code     │    result    │
   │    join       │   submit    │    update     │
   │               │             │    leaderboard│
```

## gRPC Endpoints

| Method | Description |
|--------|-------------|
| `CreateChallenge` | Create new battle room |
| `JoinChallenge` | Player joins a room |
| `SubmitCode` | Player submits code for judging |
| `GetChallengeState` | Current state of a challenge |
| `ListActiveChallenges` | All active battles |
| `GetResults` | Final results of a completed challenge |

## Quick Start

```bash
export DB_HOST=localhost DB_PORT=5432 DB_USER=zenx DB_PASS=zenx123
export CODE_ENGINE_ADDR=localhost:50054
export REDIS_ADDR=localhost:6379

go run cmd/main.go
# → gRPC + WebSocket on :50052
```

## Docker

```bash
docker build -t zenxbattle-challenge .
docker run -p 50052:50052 zenxbattle-challenge
```

## Related Services

- [CodeExecutionEngine](https://github.com/zenxbattle/CodeExecutionEngine) — sandboxed code judge
- [RedisBoard](https://github.com/zenxbattle/RedisBoard) — leaderboard engine
- [Frontend](https://github.com/zenxbattle/Frontend) — battle UI
- [ApiGateway](https://github.com/zenxbattle/ApiGateway) — REST proxy
- [CommonProto](https://github.com/zenxbattle/CommonProto) — protobuf definitions

See [lifecycle.md](./lifecycle.md) for detailed challenge state machine documentation.
