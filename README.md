# FolioRelay

FolioRelay is a Go-based research relay system built for grounded, low-latency question answering from three exact arXiv paper versions: the Transformer paper, the Moshi paper, and the Dynamic Chunking paper.

## What it includes

- Go backend with a CLI mode switch: `server`, `benchmark`, `verify`, and `fetch-papers`
- Fixed embedded corpus with structured passage metadata
- Three retrieval settings: Setting A, Setting B, and Setting C
- Calibrated stand-in SSE simulator with queueing, budget, and retry behavior
- Benchmark runner that writes `results.csv` and `REPORT.md`
- Editorial UI served from the Go server at `/`

## Run locally

```bash
go mod tidy
go run main.go --mode=benchmark
go run main.go --mode=verify
go run main.go --mode=server
```

Then open:

- `http://localhost:8080`
- `http://localhost:8080/api/metrics`

## Environment

Copy the example file and set your real key if needed. The application loads `.env` at startup without overriding variables already set in the process environment.

```bash
cp .env.example .env
```

```dotenv
GROQ_API_KEY=your_key_here
GROQ_MODEL=llama-3.1-8b-instant
PORT=8080
```

`--mode=benchmark` records 15 streamed Groq calibration requests at two-second intervals and a five-request concurrent probe when a key is configured. A model-not-found 404 triggers the configured ordered model fallback. It also writes deterministic stand-in workload and evaluation rows. If no key is configured, the calibration rows are explicitly labeled `stand-in`. Non-secret response headers and probe counts are saved in `rate_limits.json`; the API key is never written to benchmark output.

`--mode=verify` reparses `results.csv`, validates the exact columns and enum values, required load scenarios and 18 evaluation rows per retrieval setting, recomputes report tables, checks percentage and latency formatting, and regenerates `REPORT.md` if it differs.

## Architecture overview

```text
HTTP API
  ├─ /api/query
  ├─ /api/metrics
  └─ /api/simulate
       ↓
   pipeline relay
       ├─ intent + budget check
       ├─ retrieval
       ├─ sufficiency router
       ├─ synthesis + citations
       └─ verifier loop
       ↓
  embedded arXiv library + stand-in / real Groq client
```

## Benchmark outputs

The benchmark writes:

- `results.csv` — raw request log for every synthetic and calibration request
- `rate_limits.json` — non-secret headers captured from real calibration and burst responses
- `REPORT.md` — engineering post-mortem with tables and discussion
- `go run main.go --mode=verify` — mathematical consistency check and report repair

## Notes

- The embedded corpus is deterministic and works without external network access.
- Real Groq requests are used when the key is present and the API is reachable.
- The fallback stand-in enforces queueing, rate limits, and budget exhaustion behavior so the benchmark remains reproducible.
- Simulating `daily_budget_exhausted` returns the verified fallback badge and two cited reranked excerpts without making a model call.
