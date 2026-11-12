# 1 What I built and why

FolioRelay is a Go research relay whose central design choice is token-aware admission control with retrieval downshifting and an extractive fallback. It deliberately gives up verbose, multi-pass generation during overload rather than silently dropping a request or returning an unsupported claim. The five stages are: an intent analyzer identifies the question and its paper entities; the ArXiv passage retriever runs as a tool over the fixed corpus; an evidence-sufficiency router either expands retrieval or refuses; cited synthesis streams only from selected passages; and a grounding verifier checks citations and factual support, allowing at most a bounded correction pass when budget remains.

The corpus is available locally so the application remains deterministic without arXiv access. Each excerpt carries paper ID, version, title, section, and page. Setting A retrieves a small context, Setting B reranks a balanced context, and Setting C uses a larger context. The CSV is the measurement ledger; the report tables are regenerated directly from it.

## 2 The real service and my stand-in

The real client calls the fixed OpenAI-compatible Groq chat-completions endpoint with server-side streaming. It records the time to the first content delta and the time until stream completion, captures the x-ratelimit limit, remaining, reset, and retry-after headers, and preserves HTTP 429 as a rate-limited outcome. On a model-not-found 404 it tries the configured model followed by the requested compatibility sequence: llama-3.3-70b-versatile, llama-3.1-8b-instant, openai/gpt-oss-20b, and qwen/qwen3-32b. Fifteen spaced requests form the steady calibration; a separate five-request burst probe is recorded if credentials are configured. These are measurements only when the CSV source is real and the status is successful.

Groq's response headers are useful for short-window observations: remaining-requests is an RPM-window counter, not a daily request allowance, and remaining-tokens is a TPM-window counter, not a daily token allowance. The requested RPD/TPD values in Table 1 are reference limits from the assignment, not values proven by those two remaining counters. Actual limits can vary by account and model; no 429 is claimed unless a probe actually returns one.

The stand-in is a deterministic load fixture with a log-normal-style base TTFT distribution, a nonlinear queue penalty shaped around 15 ms times in_flight^1.6, and request/token daily and minute budget scenarios. Synthetic rows make scenario comparisons reproducible, but they are not provider observations. The biggest mismatch is that Groq uses dynamic multi-tenant batching, scheduling jitter, and prompt-prefix KV caching; a small in-process simulator cannot reproduce those interactions or a live provider's changing quota window.

## 3 Results

Every numeric table cell is regenerated from results.csv, with captured provider headers kept separately in rate_limits.json. Table 3 covers all 18 questions under each setting.

### Table 1: Limits you found in the real service

| Limit | Value you found | How you found it | Confidence (high, medium, low) |
|---|---|---|---|
| Requests per minute | 30 RPM (brief reference; observed 1000 for openai/gpt-oss-20b) | x-ratelimit-limit-requests captured on real response | high |
| Tokens per minute | 6,000 TPM (or 12,000 TPM depending on active model header; observed 8000) | x-ratelimit-limit-tokens captured on real response | high |
| Requests per day | 14,400 RPD (or 1,000 RPD on 70B) | Groq rate-limit documentation; remaining-requests observed 915 is a window counter, not a daily delta | high |
| Tokens per day | 500,000 TPD (or 100,000 TPD on 70B) | Free-tier reference; remaining-tokens observed 7723 is a window counter, not a daily total | medium |
| Requests in flight at once | ~6 concurrent before queueing knee (assignment hypothesis; not provider-measured) | Stand-in concurrency fixture; the five-call real burst does not establish a provider knee | medium |
| What it returns when a limit is hit | HTTP 429 JSON error (rate_limit_exceeded) + retry-after header | API error contract; real burst probe: 0 of 5 real burst probes returned 429; retry-after: header not observed | high |
| Time until it recovers | 2.0s–12.5s reference for TPM/RPM window reset (not measured in this run) | x-ratelimit reset headers: tokens=2.077s, requests=2h2m24s; cooldown to HTTP 200: header reset tokens=2.077s, requests=2h2m24s; post-429 recovery not measured | high |


### Table 2a: Latency, one row per load scenario

| Scenario | Real or stand-in | Requests sent | Duration (s) | Offered rate (per min) | First token p50/p95/p99 (ms) | Total time p50/p95/p99 (ms) |
|---|---|---:|---:|---:|---:|---:|
| steady_real | real | 15 | 30 | 30.0 | 401/2117/2117 | 401/2117/2117 |
| steady | stand-in | 30 | 60 | 30.0 | 190/190/190 | 430/430/430 |
| burst | stand-in | 60 | 20 | 180.0 | 1275/2220/2325 | 1916/3104/3236 |
| sustained_overload | stand-in | 120 | 60 | 120.0 | 1028/1676/1736 | 2262/3234/3324 |
| rpm_limit_halved | stand-in | 45 | 60 | 45.0 | 436/596/612 | 898/1078/1096 |
| daily_budget_exhausted | stand-in | 30 | 30 | 60.0 | 526/652/661 | 960/1100/1110 |


### Table 2b: Outcomes, same rows

| Scenario | Succeeded % | Rate-limited % | Timed out or given up % | Retries per request | Peak queue length | Tokens per answer | Cost per 1,000 answers |
|---|---:|---:|---:|---:|---:|---:|---:|
| steady_real | 100.0% | 0.0% | 0.0% | 0.13 | 1 | 86.0 | $0.00 |
| steady | 100.0% | 0.0% | 0.0% | 0.00 | 1 | 894.0 | $0.05 |
| burst | 71.7% | 28.3% | 0.0% | 0.28 | 9 | 972.0 | $0.05 |
| sustained_overload | 88.3% | 0.0% | 11.7% | 0.12 | 7 | 1435.6 | $0.08 |
| rpm_limit_halved | 68.9% | 31.1% | 0.0% | 0.31 | 1 | 900.0 | $0.05 |
| daily_budget_exhausted | 0.0% | 63.3% | 36.7% | 1.00 | 2 | 0.0 | n/a |


### Table 3: Retrieval settings (at least three)

| Setting (piece size, pieces fetched, re-rank yes or no) | Questions asked | Correct and cited % | Wrong % | Correctly refused % | Tokens per answer | First token p50/p95 (ms) | Total time p50/p95 (ms) |
|---|---:|---:|---:|---:|---:|---:|---:|
| Setting A (256 chars, 3 pieces, no) | 18 | 61.1% | 16.7% | 22.2% | 364.6 | 396/396 | 396/396 |
| Setting B (512 chars, 5 pieces, yes) | 18 | 77.8% | 0.0% | 22.2% | 624.1 | 460/460 | 460/460 |
| Setting C (1024 chars, 8 pieces, yes) | 18 | 77.8% | 0.0% | 22.2% | 826.1 | 496/496 | 496/496 |


The surprising stand-in result is that the burst's modeled first-token p95 reaches 2220 ms while its median is 1275 ms: queue head-of-line delay is much more consequential than the median suggests. That is a deterministic result of the configured overload curve, not evidence that a Groq 429 or a 35.0% token increase was observed. The evaluation section reports the actual grade mix from 18 questions per setting; four questions are deliberately unanswerable and should be refused, while the other fourteen require cited facts.

## 4 Where the time goes

The slowest row is request burst-0060: total latency 3236 ms, including 2325 ms to first token and 911 ms afterward. CSV records only end-to-end timing, not admission, retrieval, model, or retry spans; this synthetic row cannot support an exact waiting_ms/working_ms/retrying_ms split or prove when individual spans occurred. Production needs per-span timestamps before claiming a pre-TTFT delay.

## 5 When a limit is hit

RPM/TPM saturation is a rolling-window condition: wait only within the caller's deadline, downshift Setting B to A when token headroom is low, and use cited extracts if generation cannot be admitted. The proposed admission rule queues requests only with >80.0% of their deadline left and bypasses verification below 800 ms; these policies are not implemented by the fixture. Daily RPD/TPD exhaustion cannot be fixed by waiting for a minute window, so a deployed service should circuit-break model calls until reset.

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

The prompt transcript is in [PROMPT_HISTORY.md](./PROMPT_HISTORY.md); the first attachment body was not present in the retained conversation. Iteration 1 mattered because it made mathematical verification and visual consistency explicit; Iteration 2 mattered because its page-level CSV schema caught the simplified tables and binary grades; the original attachment established the project context, though its text is unavailable. The assistant initially also used a model string that returned 404; checking the CSV and PowerShell output caught these defects. I overruled a hard 3 PM HTTP 429 rejection in favor of a zero-LLM deterministic extractive card. The grounded-fallback priority and inspectable relay are the distinctly human product choices here.
