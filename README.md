# pgwatch-ai

`pgwatch-ai` is a CLI copilot for PostgreSQL diagnostics built on top of pgwatch metric storage.

It accepts natural-language prompts, classifies intent with an LLM, routes to metric analyzers, and returns explainable findings for operational debugging.

## What Is Implemented

The current implementation supports:

- Natural-language entrypoint:
  - `pgwatch-ai ask "<prompt>"`
- LLM-based intent classification (Gemini API):
  - intent
  - confidence
  - reasoning text
- Analyzer routing via intent
- Dynamic, sink-backed analysis for:
  - slow queries
  - connection pressure
  - replication lag
  - lock contention

All four implemented analyzers query real pgwatch sink data from PostgreSQL/Timescale-compatible storage.

## Current Architecture

Request flow:

1. CLI receives prompt (`internal/cli`)
2. Orchestrator handles request lifecycle (`internal/app/orchestrator.go`)
3. Gemini classifier maps prompt to intents (`internal/app/llm_intent_classifier.go`)
4. Router selects analyzer path (`internal/app/router.go`)
5. Analyzer reads sink metrics through reader layer (`internal/reader/sink_reader.go`)
6. Findings are returned with severity and evidence (`internal/model`)

Core modules:

- `cmd/main.go`: process entrypoint
- `internal/app`: intent detection + routing
- `internal/analysis`: analyzer implementations
- `internal/reader`: SQL-backed sink readers
- `internal/db`: connection pool setup
- `internal/model`: domain types (`Intent`, `Finding`, `ExecutionContext`, etc.)

## Implemented Intents

- `slow_queries`
- `connections`
- `replication`
- `locks`

Also recognized but not fully implemented with analyzers yet:

- `summary`
- `health_status`
- `scans`
- `explain`

## Environment Configuration

Create a local `.env` file (do not commit secrets):

```env
GEMINI_API_KEY=your_key_here
GEMINI_MODEL=gemini-2.5-flash
PWAI_SINK_DSN=postgresql://pgwatch:pgwatchadmin@postgres:5432/pgwatch_metrics?sslmode=disable
```

## Running Locally

From project root:

```bash
go mod tidy
go run ./cmd ask --sink-dsn "postgresql://pgwatch:pgwatchadmin@127.0.0.1:5432/pgwatch_metrics?sslmode=disable" "check replication lag status"
```

## Running With Docker Compose

Use the project compose file:

```bash
docker compose -f ./docker-compose.yml up --build -d
docker compose -f ./docker-compose.yml run --rm pgwatch-ai ask "show slow query issues"
```

When connecting to pgwatch services via Docker network, use service hostnames (for example `postgres`) in DSN.

## Example Prompts

- `check lock contention`
- `show slow query issues in last hour`
- `check connection pressure`
- `check replication lag status`

## Output Format (Current)

Output currently includes:

- intent reasoning (LLM-provided)
- analyzer findings with severity labels
- evidence-oriented metric summary per database



## Roadmap (Planned Next)

1. Implement sink-backed analyzers for `scans` and `health_status`
2. Add recommendation engine layer tied to findings
3. Add structured output formats (`json`) for integrations
4. Add tests for classifier parsing, router behavior, and analyzer SQL paths



