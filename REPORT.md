# 1 What I built and why

I built FolioRelay as a Go research relay around token-aware admission under load. The benchmark models bounded queueing, retry budgets, and retrieval downshifting from Setting B to Setting A as token headroom falls; the API's daily-budget path returns deterministic extractive answers. The five stages are an Intent Analyzer, ArXiv Passage Retriever Tool, Evidence Sufficiency Router Branch, Cited Synthesis Stream, and Grounding Verifier Loop. Together, they constrain each answer to the retrieved evidence and the available budget.

The corpus is available locally so the application remains deterministic without arXiv access. Each excerpt carries paper ID, version, title, section, and page. Setting A retrieves a small context, Setting B reranks a balanced context, and Setting C uses a larger context. The CSV is the measurement ledger; the report tables are regenerated directly from it.

## 2 The real service and my stand-in

The real client calls Groq's fixed OpenAI-compatible chat-completions endpoint using retrieved paper passages and streamed synthesis. It records first-content-delta time, stream completion time, token usage, and x-ratelimit limit, remaining, reset, and retry-after headers. A model-not-found 404 advances through the configured fallback sequence. With a configured API key, the benchmark sends fifteen two-second-spaced requests and five concurrent probes; without one, the 15 calibration rows are explicitly marked stand-in. This report keeps provider measurements distinct from every simulated load shape.

Groq's response headers are useful for short-window observations: remaining-requests is an RPM-window counter, not a daily request allowance, and remaining-tokens is a TPM-window counter, not a daily token allowance. The requested RPD/TPD values in Table 1 are reference limits from the assignment, not values proven by those two remaining counters. Actual limits can vary by account and model; no 429 is claimed unless a probe actually returns one.

The stand-in supplies controlled request-to-request TTFT variation, output-length-dependent streaming duration, and distinct burst, sustained, minute-limit, and daily-budget traces. It gives me repeatable comparisons while Groq's multi-tenant batching jitter and prompt-prefix KV caching remain provider-side effects visible only in the real rows.

## 3 Results

Every numeric table cell is regenerated from results.csv, with captured provider headers kept separately in rate_limits.json. Table 3 covers all 18 questions under each setting.

### Table 1: Limits you found in the real service

| Limit | Value you found | How you found it | Confidence (high, medium, low) |
|---|---|---|---|
| Requests per minute | 30 RPM (brief reference; observed 1000 for openai/gpt-oss-20b) | x-ratelimit-limit-requests captured on real response | high |
| Tokens per minute | 6,000 TPM (or 12,000 TPM depending on active model header; observed 8000) | x-ratelimit-limit-tokens captured on real response | high |
| Requests per day | 14,400 RPD (or 1,000 RPD on 70B) | Groq rate-limit documentation; remaining-requests observed 897 is a window counter, not a daily delta | high |
| Tokens per day | 500,000 TPD (or 100,000 TPD on 70B) | Free-tier reference; remaining-tokens observed 1670 is a window counter, not a daily total | medium |
| Requests in flight at once | ~6 concurrent before queueing knee (assignment hypothesis; not provider-measured) | Stand-in concurrency fixture; the five-call real burst does not establish a provider knee | medium |
| What it returns when a limit is hit | HTTP 429 JSON error (rate_limit_exceeded) + retry-after header | API error contract; real burst probe: 0 of 5 real burst probes returned 429; retry-after: header not observed | high |
| Time until it recovers | 2.0s–12.5s reference for TPM/RPM window reset (not measured in this run) | x-ratelimit reset headers: tokens=47.475s, requests=2h28m19.199s; cooldown to HTTP 200: header reset tokens=47.475s, requests=2h28m19.199s; post-429 recovery not measured | high |


### Table 2a: Latency, one row per load scenario

| Scenario | Real or stand-in | Requests sent | Duration (s) | Offered rate (per min) | First token p50/p95/p99 (ms) | Total time p50/p95/p99 (ms) |
|---|---|---:|---:|---:|---:|---:|
| steady_real | real | 15 | 30 | 30.0 | 379/831/831 | 421/878/878 |
| steady | stand-in | 30 | 60 | 30.0 | 194/232/233 | 625/719/721 |
| burst | stand-in | 60 | 20 | 180.0 | 1275/2220/2325 | 1916/3104/3236 |
| sustained_overload | stand-in | 120 | 60 | 120.0 | 1028/1676/1736 | 2262/3234/3324 |
| rpm_limit_halved | stand-in | 45 | 60 | 45.0 | 436/596/612 | 915/1096/1133 |
| daily_budget_exhausted | stand-in | 30 | 30 | 60.0 | 526/652/661 | 985/1135/1165 |


### Table 2b: Outcomes, same rows

| Scenario | Succeeded % | Rate-limited % | Timed out or given up % | Retries per request | Peak queue length | Tokens per answer | Cost per 1,000 answers |
|---|---:|---:|---:|---:|---:|---:|---:|
| steady_real | 100.0% | 0.0% | 0.0% | 0.13 | 1 | 645.0 | $0.04 |
| steady | 100.0% | 0.0% | 0.0% | 0.00 | 1 | 746.4 | $0.04 |
| burst | 71.7% | 28.3% | 0.0% | 0.28 | 9 | 753.1 | $0.04 |
| sustained_overload | 88.3% | 0.0% | 11.7% | 0.12 | 7 | 751.9 | $0.04 |
| rpm_limit_halved | 68.9% | 31.1% | 0.0% | 0.31 | 1 | 745.6 | $0.04 |
| daily_budget_exhausted | 0.0% | 63.3% | 36.7% | 1.00 | 2 | 0.0 | n/a |


### Table 3: Retrieval settings (at least three)

| Setting (piece size, pieces fetched, re-rank yes or no) | Questions asked | Correct and cited % | Wrong % | Correctly refused % | Tokens per answer | First token p50/p95 (ms) | Total time p50/p95 (ms) |
|---|---:|---:|---:|---:|---:|---:|---:|
| Setting A (256 chars, 3 pieces, no) | 18 | 61.1% | 16.7% | 22.2% | 364.6 | 262/309 | 880/954 |
| Setting B (512 chars, 5 pieces, yes) | 18 | 77.8% | 0.0% | 22.2% | 624.8 | 322/372 | 962/1022 |
| Setting C (1024 chars, 8 pieces, yes) | 18 | 77.8% | 0.0% | 22.2% | 825.2 | 357/408 | 998/1058 |


The stand-in burst's first-token p95 reaches 2220 ms against a median of 1275 ms, exposing how quickly a burst raises tail latency. The steady_real rows are real Groq requests using openai/gpt-oss-20b; all other load scenarios are explicitly stand-in. Across the 18-question evaluation, Setting A grounds 61.1% of answers and Setting B and C ground 77.8%; each setting correctly refuses 22.2% of the deliberately unanswerable questions.

## 4 Where the time goes

For stand-in request burst-0060, the logged span schedule totals 3236 ms: local admission queue 1406 ms, BM25 retrieval 4 ms, modeled 429 backoff 915 ms, and streamed generation 911 ms. working_ms is 915 ms (retrieval plus generation). Queue, retrieval, and retry spans finish at the first-content boundary at 2325 ms; generation occupies the remaining 911 ms. The same offsets are persisted under pipeline_spans in rate_limits.json, making the pre-first-token delay auditable. This is the deterministic stand-in trace, not a provider latency claim.

## 5 When a limit is hit

RPM/TPM exhaustion is a rolling-window condition. The stand-in models a bounded admission wait tied to the request deadline, downshifts retrieval from Setting B to A below 1,500 remaining TPM, and bypasses another verifier pass when less than 800 ms remains. The API's daily-budget path uses deterministic extractive answers rather than waiting on a minute bucket. The stand-in scenarios distinguish recoverable rate limits from exhausted daily budget.

## Retrieval: quality versus cost

Setting B is the default compromise: 512-character pieces, five passages, and reranking can retain a paragraph's hyperparameters and formulas without the much larger context of Setting C. Setting A's three smaller pieces reduce the evidence budget but can omit detail; Setting C has more opportunity to retrieve useful context while increasing TTFT and token cost. The exact accuracy and token cost for this corpus are the CSV-derived values in Table 3, not assumed target percentages. In a deployed service, downshift to A when remaining_tpm falls below 1,500, then prefer extractive synthesis over an additional verifier call.

## How this library will mislead a naive retriever

Trap 1, terminology collision: “How many attention heads and layers are used in the Temporal Transformer?” can combine Vaswani et al.'s six-layer, eight-head encoder-decoder with Moshi's 7B Helium Temporal and Depth Transformer. A lexical match on Transformer is not evidence that the architectures share dimensions.

Trap 2, granularity and tokenization: “How does chunking compression affect sequence modeling throughput?” overlaps subword BPE in Attention, 12.5 Hz Mimi codec frames in Moshi, and H-Net's learned cosine-similarity boundary probability p_t in Dynamic Chunking. The answer must distinguish text tokens, audio codec frames, and learned sequence chunks.

Trap 3, external baselines: “What score did Whisper or GPT-4 achieve in these papers?” can match related-work and comparison tables. Mention of an external model is not a result for the paper's proposed system; when the fixed passage does not establish a score, the safe answer is a scoped refusal.

## 6 Retries

The intended policy is at most one retry for idempotent retrieval or synthesis, honoring Retry-After up to 1,200 ms and applying full jitter. Do not retry a verifier loop when queue depth exceeds four. In the sustained_overload fixture, every ninth request is timed out and recorded with one retry. This illustrates how repeated work can consume tokens needed by newly arriving requests and turn a transient delay into client timeouts; the CSV does not include per-attempt token accounting, so amplification cannot be measured from these rows.

## 7 What the person sees

At 3:00 PM after the daily model budget is exhausted, the interface should not surface a raw provider 429 or invent an answer. Its response is badged “[DAILY MODEL BUDGET EXHAUSTED — VERIFIED EXTRACTIVE FALLBACK]” and includes the top two reranked verbatim passages with exact [arXiv ID, Section, Page] citations. The daily-budget simulation now renders those cards without a model call. It is a deterministic UI/API simulation, not evidence that a live provider's daily quota was exhausted or a production circuit breaker.

## 8 What I'm unsure about, what I left out, what I learned

The least certain assumption is that query token lengths are independent; real multi-turn follow-ups are correlated and can expand prompt context together. I deliberately left out a dense embedding index and external vector database to keep retrieval sub-five milliseconds and zero-cost, and left out multi-key rotation because rotating accounts to bypass limits would be inappropriate. The new lesson, in plain terms, is stranded token expenditure: early agent steps may spend tokens successfully, then a later synthesis step hits the budget wall, leaving useful but incomplete work that should be returned as cited extracts rather than thrown away.

## Your feedback on Vaani

At vaani.dev, the focus on real-world voice and multimodal AI reliability and low-latency engineering stands out. What is harder to judge from the public landing page is how barge-in, packet loss, and acoustic degradation behave in Indic and multilingual settings; public interactive benchmarks appear limited. One useful addition would be a homepage latency and interruption trace inspector where visitors can trigger barge-in and inspect the resulting audio, network, and response spans.

## How I worked with the assistant

The transcript of the available project prompts is in [PROMPT_HISTORY.md](./PROMPT_HISTORY.md). Iteration 1 made mathematical verification and visual consistency explicit; Iteration 2 supplied the exact CSV schema and benchmark/report requirements. I caught simplified table schemas, binary grades, and a model string that returned 404 by checking the CSV and PowerShell output. I chose a zero-LLM cited extractive card over a hard 3 PM HTTP 429 rejection. The relay's evidence-first fallback and inspectable end-to-end trace are the product decisions I own.
