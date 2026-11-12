# Prompt history

The earliest user message in the retained conversation was `#attachment:Pasted text #1`. Its attachment body is not available in the retained transcript, so it cannot be reproduced here. The following are the complete subsequent user prompts.

## Iteration 1

> # ITERATION 1: Automated Self-Audit, Exact UI Polish & Git Integration
>
> Execute this entire workflow autonomously from start to finish. Do not ask for confirmation—run the edits, run the terminal verification commands, fix any discrepancies yourself until all checks pass, and output the final verification summary.
>
> ### STEP 1: Load Groq Key (if present) & Verify Real + Stand-in Calibration
> 1. Check if `.env` exists or if `GROQ_API_KEY` is set in the environment. Ensure `main.go` automatically loads `.env` (parse `KEY=VALUE` lines from `.env` at startup so `GROQ_API_KEY` works seamlessly without needing manual PowerShell export).
> 2. Ensure `.gitignore` exists and strictly excludes `.env`, `*.exe`, and temporary build artifacts so the API key is NEVER committed to Git.
>
> ### STEP 2: Build Automated Mathematical Self-Auditor (`--mode=verify`)
> Reviewers run an automated script on `results.csv` to recompute every cell in Table 2a, Table 2b, and Table 3 of `REPORT.md`.
> 1. Add `--mode=verify` to `main.go` and `internal/benchmark/runner.go`.
> 2. When `--mode=verify` runs, it must parse `results.csv` from disk and independently check:
>    - Exact column headers: `request_id,scenario,source,start_time,first_token_ms,total_ms,status,retries,tokens_in,tokens_out,retrieval_setting,answer_grade`
>    - Every scenario required by Table 2a/2b (`steady`, `burst`, `sustained_overload`, `rpm_limit_halved`, `daily_budget_exhausted`) is present.
>    - Every retrieval setting required by Table 3 (`Setting A`, `Setting B`, `Setting C`) is present with at least 15 questions each.
>    - All percentages in `REPORT.md` use **one decimal place** (e.g., `93.3%`, `0.0%`) and all latencies are integer milliseconds (`ms`).
>    - Recompute `p50`, `p95`, `p99` for `first_token_ms` and `total_ms`, success %, rate-limited %, timeout %, retries/request, and tokens/answer directly from `results.csv` and assert they match the exact strings written inside `REPORT.md`. If any number differs, automatically regenerate `REPORT.md` from `results.csv` so divergence is mathematically impossible.
>
> ### STEP 3: Polish `web/index.html` to 100% Match "The Curious Corner"
> Update `web/index.html` and inspect the DOM structure to guarantee it matches the 3 reference screenshots of `curious-corner.vercel.app`:
> 1. **Color & Surface Rules:**
>    - Background `#FAF7F2`, Featured Hero Card `#EFECE6`, Inner Signal Canvas `#E2DDD5`, Text `#231F1D`, Muted Text `#7A7267`, Burnt Sienna Accent `#A66A43`, Hairline Borders `1px solid #E2DDD5`, Footer `#26201B`.
>    - Zero drop-shadows (`box-shadow: none !important`), zero generic bootstrap/dashboard rounded cards (`border-radius: 0px` on all cards and containers, except circular radar rings).
> 2. **Typography & Exact Sections:**
>    - Google Fonts: `'Playfair Display'` (400, 400i, 500) + `'Plus Jakarta Sans'` (400, 500, 600) + `'JetBrains Mono'` (400).
>    - **Header:** Left brand `FolioRelay.` (with `#A66A43` period). Right nav: `Discover`, `Library`, `Telemetry`, `Brief`. Bottom border `1px solid #E2DDD5`.
>    - **Hero Split Section:**
>      - Left: `—— A LITTLE LATENCY GOES A LONG WAY` above giant serif heading `The relay is more resilient than you ` + `<em style="color:#A66A43; font-style:italic;">think.</em>`
>      - Right (`#EFECE6` card): Top row `THE CURIOUS FILE` (left) and italic `No. 01` (right). Center `#E2DDD5` canvas with the concentric circle diagram (`Wi` / `RL` center `#5C4938` circle, 3 thin concentric rings, 2 orbiting dots, and `SIGNAL` label). Below canvas: `INFERENCE · 5 STEP PIPELINE` and serif title `What actually happens when a burst hits the relay?`
>    - **3-Column Editorial Grid ("Ideas worth *exploring.*"):**
>      - Exact layout from screenshot 2: Top kicker `A COLLECTION OF CURATED PAPERS`, large serif heading `Ideas worth ` + `<em style="color:#A66A43; font-style:italic;">exploring.</em>` with `View all metrics →` on the right.
>      - 3 columns (`01`, `02`, `03` in large `#C8BFB3` serif) for `1706.03762v7`, `2410.00037v1`, and `2507.07955v2`, plus interactive query execution, live 5-step trace, and Tables 1, 2a, 2b, 3 rendered cleanly in the same editorial aesthetic.
>    - **Dark Espresso Footer (`#26201B`):**
>      - Left column: `FolioRelay.`, description, and italic `#A66A43` sign-off `Stay grounded. Always.`
>      - Middle column: `EXPLORE` (`Interactive Relay`, `Paper Corpus`, `Load Telemetry`).
>      - Right column: `THE LITTLE THINGS` (`Attention 1706.03762v7`, `Moshi 2410.00037v1`, `Dynamic Chunking 2507.07955v2`, `Rate Limit Policies`).
>      - Bottom bar: `© 2026 FolioRelay. All rights reserved.` on left, italic `Made for the endlessly curious.` on right.
>
> ### STEP 4: Configure Git Repository & Run Automated Smoke + Audit Suite
> 1. Initialize git if not already initialized (`git init`), set branch to `main` (`git branch -M main`),

## Iteration 2 (Final)

> # ITERATION 2 (FINAL): Exact PDF Schema Compliance, Real Groq Fix, Full 2-Page Report & Git Push
>
> Execute this entire workflow autonomously from start to finish. Do not simplify or rename ANY table columns or CSV enum values—the grading script checks exact column headers and string values from the assignment brief.
>
> ### STEP 1: Fix Real Groq API Call & Header Inspection (`internal/llm/client.go`)
> 1. When loading `.env`, apply `strings.TrimSpace` and strip any surrounding quotes (`"` or `'`) from `GROQ_API_KEY` and `GROQ_MODEL`.
> 2. Ensure the endpoint is strictly `https://api.groq.com/openai/v1/chat/completions`.
> 3. If the primary model returns HTTP 404 or model-not-found, automatically try this fallback list in order until one succeeds with HTTP 200:
>    - `llama-3.3-70b-versatile`
>    - `llama-3.1-8b-instant`
>    - `openai/gpt-oss-20b`
>    - `qwen/qwen3-32b`
> 4. During `--mode=benchmark`, send **15 real requests** in the `steady (real)` scenario (spaced out or with modest concurrency so most succeed and capture real `first_token_ms`, `total_ms`, and `x-ratelimit-*` headers, plus a brief burst probe to observe/verify real 429 behavior if triggered). Record these real rows in `results.csv` with `source=real`.
>
> ### STEP 2: Enforce Exact `results.csv` Enum Values (`internal/benchmark/runner.go`)
> Page 4 of the brief strictly mandates:
> - Columns: `request_id,scenario,source,start_time,first_token_ms,total_ms,status,retries,tokens_in,tokens_out,retrieval_setting,answer_grade`
> - `source` MUST be strictly `real` or `stand-in`
> - `status` MUST be strictly one of: `ok`, `rate_limited`, `timeout`, `error`
> - `answer_grade` MUST be strictly one of: `correct`, `wrong`, `refused_correctly`, `refused_wrongly` (NEVER `0` or `1`!).
>   - Ensure our 18 evaluation questions include 14 answerable paper questions and 4 unanswerable trap questions.
>   - Tune the retrieval & grading pipeline so:
>     - `Setting A (256 chars, 3 pieces, rerank=no)` achieves ~66.7% `correct`, ~22.2% `refused_correctly`, ~11.1% `wrong`
>     - `Setting B (512 chars, 5 pieces, rerank=yes)` achieves ~72.2% `correct`, ~22.2% `refused_correctly`, ~5.6% `wrong`
>     - `Setting C (1024 chars, 8 pieces, rerank=yes)` achieves ~72.2% `correct`, ~22.2% `refused_correctly`, ~5.6% `wrong` (at nearly 2x the input token cost and higher TTFT).
>
> ### STEP 3: Enforce Exact Required Markdown Tables in `REPORT.md` & `--mode=verify`
> Update `internal/benchmark/runner.go` so `REPORT.md` is generated with the **EXACT** tables and columns required by Page 3 of the brief (all numbers computed directly from `results.csv`, formatted in `ms` for time and `1 decimal place` for percentages):
>
> 1. **Table 1: Limits you found in the real service**
>    Columns:
>    `| Limit | Value you found | How you found it | Confidence (high, medium, low) |`
>    Rows required:
>    - `Requests per minute` (`30 RPM`, `Groq response header x-ratelimit-limit-requests & 429 probe + docs`, `high`)
>    - `Tokens per minute` (`6,000 TPM (or 12,000 TPM depending on active model header)`, `x-ratelimit-limit-tokens header on real responses`, `high`)
>    - `Requests per day` (`14,400 RPD (or 1,000 RPD on 70B)`, `Groq rate-limit documentation + x-ratelimit-remaining-requests delta`, `high`)
>    - `Tokens per day` (`500,000 TPD (or 100,000 TPD on 70B)`, `Groq free-tier documentation + daily token counter tracking`, `medium`)
>    - `Requests in flight at once` (`~6 concurrent before queueing knee`, `Empirical concurrent burst probe measuring TTFT inflection from ~210ms to >680ms`, `medium`)
>    - `What it returns when a limit is hit` (`HTTP 429 JSON error (rate_limit_exceeded) + retry-after header`, `Triggered via burst probe exceeding TPM/RPM bucket`, `high`)
>    - `Time until it recovers` (`2.0s–12.5s for TPM/RPM window reset ( matches x-ratelimit-reset-tokens )`, `Measured cooldown until subsequent probe returned HTTP 200`, `high`)
>
> 2. **Table 2a: Latency, one row per load scenario**
>    Columns:
>    `| Scenario | Real or stand-in | Requests sent | Duration (s) | Offered rate (per min) | First token p50/p95/p99 (ms) | Total time p50/p95/p99 (ms) |`
>    Include rows for:
>    - `steady_real` (`real`, 15 requests, 30s, 30.0/min, ...)
>    - `steady` (`stand-in`, 30 requests, 60s, 30.0/min, ...)
>    - `burst` (`stand-in`, 60 requests, 20s, 180.0/min, ...)
>    - `sustained_overload` (`stand-in`, 120 requests, 60s, 120.0/min, ...)
>    - `rpm_limit_halved` (`stand-in`, 45 requests, 60s, 45.0/min, ...)
>    - `daily_budget_exhausted` (`stand-in`, 30 requests, 30s, 60.0/min, ...)
>
> 3. **Table 2b: Outcomes, same rows**
>    Columns:
>    `| Scenario | Succeeded % | Rate-limited % | Timed out or given up % | Retries per request | Peak queue length | Tokens per answer | Cost per 1,000 answers |`
>    (Compute every percentage to 1 decimal place from `results.csv` for the exact same rows as Table 2a; compute Cost per 1,000 answers at Groq reference pricing `$0.05/1M in, $0.08/1M out` -> e.g. `$0.04`–`$0.09`.)
>
> 4. **Table 3: Retrieval settings (at least three)**
>    Columns:
>    `| Setting (piece size, pieces fetched, re-rank yes or no) | Questions asked | Correct and cited % | Wrong % | Correctly refused % | Tokens per answer | First token p50/p95 (ms) | Total time p50/p95 (ms) |`
>    Include rows for:
>    - `Setting A (256 chars, 3 pieces, no)` (18 questions, ...)
>    - `Setting B (512 chars, 5 pieces, yes)` (18 questions, ...)
>    - `Setting C (1024 chars, 8 pieces, yes)` (18 questions, ...)
>
> ### STEP 4: Write the Complete, Deep 2-Page Engineering Narrative in `REPORT.md`
> Replace the short 1-sentence stubs in `REPORT.md` with a substantive, concrete 2-page systems engineering report (~1,100–1,400 words) using the **exact required headings in order**:
> 1. `# 1 What I built and why` — Explain the 5-stage Go pipeline (`Intent Analyzer -> ArXiv Passage Retriever Tool -> Evidence Sufficiency Router Branch -> Cited Synthesis Stream -> Grounding Verifier Loop`) and the single architectural decision that shaped it most: **Token-Aware Admission Control with Retrieval Downshifting & Extractive Fallback** (sacrificing verbose multi-pass LLM generation under overload so we never hallucinate or drop users silently).
> 2. `## 2 The real service and my stand-in` — Detail the real Groq API measurements, how `x-ratelimit-*` headers behave (specifically that Groq exposes `x-ratelimit-remaining-requests` for RPD and `x-ratelimit-remaining-tokens` for TPM), how the Go Stand-in Simulator is calibrated (log-normal base TTFT + non-linear queuing curve `15ms * in_flight^1.6` + sliding window RPM/TPM/RPD buckets), and where the stand-in differs most from the real service (Groq's dynamic multi-tenant batching jitter and prompt-prefix KV caching effects).
> 3. `## 3 Results` — Render Tables 1, 2a, 2b, and 3. Highlight the **surprising empirical number we could not have guessed without running it** (e.g., during `burst`, retrying multi-step pipeline calls eats TPM budget on step 1/3 even when step 4 gets rate-limited, causing effective token consumption per completed answer to jump by >35% while queue head-of-line blocking pushes p95 TTFT past 1,800ms). Summarize performance across the 18 evaluation questions.
> 4. `## 4 Where the time goes` — Take a single concrete slow request ID from `results.csv` (e.g., from `sustained_overload` or `burst`), break its exact total latency in `ms` into `waiting_ms` (local admission queue + token-bucket wait), `working_ms` (retrieval + model inference compute), and `retrying_ms` (backoff after 429), and state whether the delay occurred **before or after the first token** (before TTFT, proven by pipeline span timestamps logged prior to SSE chunk #1).
> 5. `## 5 When a limit is hit` — Contrast per-minute (RPM/TPM) vs per-day (RPD/TPD) exhaustion: at RPM/TPM saturation, requests with >80% remaining client deadline wait in a bounded priority queue while retrieval automatically downshifts from `Setting B` to `Setting A` to cut input tokens by 42%; interactive queries with <800ms deadline budget bypass the LLM verifier loop. At the per-day limit, LLM calls are circuit-broken immediately to zero.
> 6. `## Retrieval: quality versus cost` — Analyze Table 3 across Settings A, B, and C. Explain why `Setting B (512 chars, k=5, BM25+overlap reranked)` is the production default (captures full hyperparameter tables and architectural formulas that `Setting A` truncates, while avoiding the 1.8x token bloat and TPM exhaustion of `Setting C`), and explain automatic downshifting to `Setting A` when `remaining_tpm < 1,500`.
> 7. `## How this library will mislead a naive retriever` — Detail 3 concrete failure modes with exact example questions:
>    - **Trap 1 (Terminology Collision across Architectures):** Asking *"How many attention heads and layers are used in the Temporal Transformer?"* misleads a naive retriever into merging Vaswani et al.'s 6-layer, 8-head encoder-decoder (`1706.03762v7`) with Moshi's 7B Helium Temporal + Depth Transformer (`2410.00037v1`).
>    - **Trap 2 (Granularity & Tokenization Ambiguity):** Asking *"How does chunking compression affect sequence modeling throughput?"* causes lexical overlap between subword BPE token counts in *Attention*, 12.5 Hz Mimi audio codec frames in *Moshi*, and learned cosine-similarity boundary probability $p_t$ in *Dynamic Chunking* (`2507.07955v2`).
>    - **Trap 3 (External Baselines Mentioned Inside Papers):** Asking *"What score did Whisper or GPT-4 achieve in these papers?"* tricks a naive keyword retriever because competing models appear in related-work/baseline tables, tempting the generator to cite a baseline number as the paper's own architecture or hallucinate missing benchmarks.
> 8. `## 6 Retries` — State the bounded retry policy (max 1 retry for idempotent retrieval/synthesis steps, respecting `Retry-After` up to 1,200ms with full jitter, zero retries on verifier loops when queue depth > 4) and show a concrete case from `sustained_overload` where retrying made tail latency and goodput worse (retry storm consumed tokens from newly arriving requests, turning a transient 2-second spike into a cascading client-timeout wave).
> 9. `## 7 What the person sees` — Show the exact verbatim message an engineer receives at 3:00 PM when the daily budget is exhausted: instead of a raw 429 error or an unverified hallucination, FolioRelay switches to **Deterministic Extractive Mode**, returning a clearly badged banner (`[DAILY MODEL BUDGET EXHAUSTED — VERIFIED EXTRACTIVE FALLBACK]`) followed by the top-2 reranked verbatim paper excerpts with exact `[arXiv ID, Section, Page]` citations.
> 10. `## 8 What I'm unsure about, what I left out, what I learned` — Least certain assumption (that user queries arrive with independent token lengths rather than correlated multi-turn follow-ups that blow out prompt context simultaneously); two things deliberately left out (dense vector embeddings/external vector DBs to keep retrieval sub-5ms and zero-cost, and multi-key provider rotation since ground rules forbid bypassing single-account limits); one new lesson explained in plain words (how multi-step agent pipelines suffer from "stranded token expenditure" when step 1 succeeds but step 4 hits a TPM wall).
> 11. `## Your feedback on Vaani` — Honest, specific product critique of `https://www.vaani.dev/` (what stood out: focus on real-world voice/multimodal AI reliability and low-latency engineering; what confused: limited public interactive demos/benchmarks on the landing page showing how Vaani handles barge-in, packet loss, or acoustic degradation in Indic/multilingual contexts; one change: add an interactive latency & interruption trace inspector right on the homepage).
> 12. `## How I worked with the assistant` — Reference `PROMPT_HISTORY.md` (and create `PROMPT_HISTORY.md` in the repo root with the full transcript of our prompts), list the **3 prompts that mattered most and why**, **1 bug the assistant introduced that I caught** (the assistant initially emitted simplified Table 2/3 schemas and binary `0/1` grades in `results.csv` plus a 404 model string, which I caught by auditing `results.csv` against Page 3 & 4 of the brief and inspecting the PowerShell verification output), **1 decision where I overruled it** (overruling a hard HTTP 429 rejection at 3 PM in favor of a zero-LLM deterministic extractive passage card so engineers still get cited paper snippets), and what is distinctly mine.
>
> ### STEP 5: Update UI (`web/index.html`) to Render the Exact Tables + Run Full Audit + Push to GitHub
> 1. Ensure `/api/metrics` and `web/index.html` render the updated metrics and allow downloading `results.csv`.
> 2. Run:
>    - `go fmt ./...`
>    - `go build ./...`
>    - `go run main.go --mode=benchmark`
>    - `go run main.go --mode=verify`
> 3. Verify `git remote -v` points to `https://github.com/kritik8/folio-relay.git` (set it if missing).
> 4. Ensure `.env` is ignored (`git check-ignore -v .env`), stage all project files (`git add .`), commit with message `"feat: complete FolioRelay pipeline, calibrated simulator, editorial UI, and verified benchmark report"`, and push to `origin main` (`git push -u origin main`).
> 5. Print the output of `go run main.go --mode=verify`, `git status`, and the first 120 lines of `REPORT.md`.
