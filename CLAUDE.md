# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o server main.go   # Build the server binary
go test ./...                # Run tests
go vet ./...                 # Static analysis
go fmt ./...                 # Format code
```

The server runs on port 8080 (or `$PORT` env var) and requires a PostgreSQL connection (`DATABASE_URL` env var, falls back to local postgres).

## Architecture

Gin framework + PostgreSQL (`lib/pq`) + WebSockets (`gorilla/websocket`).

```
imposter-api/
├── main.go              # Bootstrap: init DB, create Hub, register routes, start server
├── db/db.go             # DB connection and migrations
├── models/review.go     # Review struct + DB queries
├── handlers/
│   ├── review.go        # HTTP handler: POST /reviews
│   ├── room.go          # HTTP handlers: POST /rooms, POST /rooms/:code/join
│   └── ws.go            # WebSocket handler: GET /ws/:code
├── ws/
│   ├── hub.go           # Hub (all rooms), Room, Player structs + game logic
│   └── messages.go      # Message types and payload structs
└── routes/routes.go     # Route registration
```

**HTTP Endpoints**:
- `GET /ping` — health check
- `POST /reviews` — submit a review (description + star rating 1–5)
- `POST /rooms` — create a room; body: `{"host_name": "..."}` → returns `room_code`, `player_id`
- `POST /rooms/:code/join` — join a room; body: `{"name": "..."}` → returns `player_id`

**WebSocket**:
- `GET /ws/:code?player_id=<uuid>` — connect after joining a room

**WS message flow** (client → server):
- `start_game` (host only) — payload: `{word, hint, imposters_count}`
- `ready` — player finished reading their card
- `end_discussion` (host only) — advance to voting
- `vote` — payload: `{voted_player_id}`

**WS message flow** (server → client):
- `player_joined` / `player_left` — lobby updates with full player list
- `role_assigned` — citizens get `word`+`hint`, impostors get only `hint`
- `phase_discussion` — with `starting_player_name` and `duration` (300s)
- `phase_voting` — start voting
- `vote_update` — `{votes_cast, total_players}`
- `game_result` — `{voted_out_name, was_imposter, imposters[], word}`

**Game phases**: `waiting` → `card_reveal` → `discussion` → `voting` → `result`

**Room lifecycle**: rooms are in-memory (Hub). Empty rooms are deleted automatically when the last player disconnects.
