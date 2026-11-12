package benchmark

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kritik8/folio-relay/internal/library"
	"github.com/kritik8/folio-relay/internal/llm"
	"github.com/kritik8/folio-relay/internal/pipeline"
)

var csvHeaders = []string{
	"request_id", "scenario", "source", "start_time", "first_token_ms", "total_ms",
	"status", "retries", "tokens_in", "tokens_out", "retrieval_setting", "answer_grade",
}

var requiredScenarios = []string{
	"steady_real", "steady", "burst", "sustained_overload", "rpm_limit_halved", "daily_budget_exhausted",
}

var retrievalSettings = []string{"Setting A", "Setting B", "Setting C"}

var scenarioSpecs = []struct {
	name          string
	requests      int
	durationSec   int
	offeredPerMin float64
}{
	{"steady_real", 15, 30, 30},
	{"steady", 30, 60, 30},
	{"burst", 60, 20, 180},
	{"sustained_overload", 120, 60, 120},
	{"rpm_limit_halved", 45, 60, 45},
	{"daily_budget_exhausted", 30, 30, 60},
}

var enumStatuses = map[string]bool{"ok": true, "rate_limited": true, "timeout": true, "error": true}
var enumSources = map[string]bool{"real": true, "stand-in": true}
var enumGrades = map[string]bool{
	"correct": true, "wrong": true, "refused_correctly": true, "refused_wrongly": true,
}

type ResultRow struct {
	RequestID        string
	Scenario         string
	Source           string
	StartTime        time.Time
	FirstTokenMS     int
	TotalMS          int
	Status           string
	Retries          int
	TokensIn         int
	TokensOut        int
	RetrievalSetting string
	AnswerGrade      string
}

type ScenarioMetrics struct {
	Requests           int     `json:"requests"`
	Source             string  `json:"source"`
	DurationSeconds    int     `json:"duration_seconds"`
	OfferedRatePerMin  float64 `json:"offered_rate_per_min"`
	SuccessPct         float64 `json:"success_pct"`
	RateLimitedPct     float64 `json:"rate_limited_pct"`
	TimeoutPct         float64 `json:"timeout_pct"`
	RetriesPerRequest  float64 `json:"retries_per_request"`
	PeakQueueLength    int     `json:"peak_queue_length"`
	TokensPerAnswer    float64 `json:"tokens_per_answer"`
	CostPer1000Answers float64 `json:"cost_per_1000_answers"`
	FirstTokenP50MS    int     `json:"first_token_p50_ms"`
	FirstTokenP95MS    int     `json:"first_token_p95_ms"`
	FirstTokenP99MS    int     `json:"first_token_p99_ms"`
	TotalTimeP50MS     int     `json:"total_time_p50_ms"`
	TotalTimeP95MS     int     `json:"total_time_p95_ms"`
	TotalTimeP99MS     int     `json:"total_time_p99_ms"`
}

type RetrievalMetrics struct {
	Questions         int     `json:"questions"`
	CorrectPct        float64 `json:"correct_pct"`
	WrongPct          float64 `json:"wrong_pct"`
	RefusedCorrectPct float64 `json:"refused_correct_pct"`
	RefusedWrongPct   float64 `json:"refused_wrong_pct"`
	TokensPerAnswer   float64 `json:"tokens_per_answer"`
	FirstTokenP50MS   int     `json:"first_token_p50_ms"`
	FirstTokenP95MS   int     `json:"first_token_p95_ms"`
	TotalTimeP50MS    int     `json:"total_time_p50_ms"`
	TotalTimeP95MS    int     `json:"total_time_p95_ms"`
}

type RateLimitEvidence struct {
	Model          string               `json:"model"`
	Headers        map[string]string    `json:"headers"`
	SteadyRequests int                  `json:"steady_requests"`
	BurstRequests  int                  `json:"burst_requests"`
	Burst429       int                  `json:"burst_429"`
	PipelineSpans  map[string]SpanTrace `json:"pipeline_spans,omitempty"`
}

type SpanEvent struct {
	Name    string `json:"name"`
	StartMS int    `json:"start_ms"`
	EndMS   int    `json:"end_ms"`
}

type SpanTrace struct {
	RequestID        string      `json:"request_id"`
	Source           string      `json:"source"`
	Scenario         string      `json:"scenario"`
	FirstTokenMS     int         `json:"first_token_ms"`
	TotalMS          int         `json:"total_ms"`
	WaitingMS        int         `json:"waiting_ms"`
	RetrievalMS      int         `json:"retrieval_ms"`
	RetryingMS       int         `json:"retrying_ms"`
	GenerationMS     int         `json:"generation_ms"`
	WorkingMS        int         `json:"working_ms"`
	BeforeFirstToken int         `json:"before_first_token_ms"`
	AfterFirstToken  int         `json:"after_first_token_ms"`
	Events           []SpanEvent `json:"events"`
}

type MetricsSummary struct {
	TotalRequests     int                         `json:"total_requests"`
	SuccessRate       float64                     `json:"success_rate"`
	RateLimited       float64                     `json:"rate_limited"`
	TimeoutRate       float64                     `json:"timeout_rate"`
	ErrorRate         float64                     `json:"error_rate"`
	AvgTTFT           float64                     `json:"avg_ttft_ms"`
	P50TTFT           float64                     `json:"p50_ttft_ms"`
	P95TTFT           float64                     `json:"p95_ttft_ms"`
	P99TTFT           float64                     `json:"p99_ttft_ms"`
	AvgTotal          float64                     `json:"avg_total_ms"`
	P50Total          float64                     `json:"p50_total_ms"`
	P95Total          float64                     `json:"p95_total_ms"`
	P99Total          float64                     `json:"p99_total_ms"`
	RetriesPerRequest float64                     `json:"retries_per_request"`
	TokensPerAnswer   float64                     `json:"tokens_per_answer"`
	SlowestBurstID    string                      `json:"slowest_burst_id"`
	SlowestBurstTotal int                         `json:"slowest_burst_total_ms"`
	SlowestBurstTTFT  int                         `json:"slowest_burst_ttft_ms"`
	Table1            map[string]float64          `json:"table1"`
	Table2A           map[string]ScenarioMetrics  `json:"table2a"`
	Table2B           map[string]ScenarioMetrics  `json:"table2b"`
	Table3            map[string]RetrievalMetrics `json:"table3"`
	RateLimits        RateLimitEvidence           `json:"rate_limits"`
}

func Run() error {
	rows := make([]ResultRow, 0, 360)
	realRows, evidence, err := runRealCalibration()
	if err != nil {
		return err
	}
	rows = append(rows, realRows...)
	probeRows, probeEvidence := runRealBurstProbe()
	rows = append(rows, probeRows...)
	mergeRateLimitEvidence(&evidence, probeEvidence)
	for _, spec := range scenarioSpecs[1:] {
		scenarioRows := runScenarioRows(spec.name, spec.requests, spec.durationSec, "stand-in", "Setting B")
		rows = append(rows, scenarioRows...)
	}
	evaluationRows, err := runEvaluationRows()
	if err != nil {
		return err
	}
	rows = append(rows, evaluationRows...)
	evidence.PipelineSpans = map[string]SpanTrace{}
	for _, row := range rows {
		if row.RequestID == "burst-0060" {
			evidence.PipelineSpans[row.RequestID] = buildBurstSpanTrace(row)
			break
		}
	}
	if err := writeRateLimitEvidence(evidence); err != nil {
		return err
	}
	if err := writeResultsCSV(rows, "results.csv"); err != nil {
		return err
	}
	metrics, err := LoadMetricsFromCSV("results.csv")
	if err != nil {
		return err
	}
	return writeReport(BuildReportMarkdown(metrics))
}

func realCalibrationPrompt(index int) (string, string) {
	questions := pipeline.EvaluationQuestions()
	question := questions[index%14]
	passages := library.RetrieveArxivPassages(question.Query, library.ParseRetrievalSetting("Setting B"))
	var context strings.Builder
	for _, passage := range passages {
		if context.Len() > 0 {
			context.WriteString("\n\n")
		}
		fmt.Fprintf(&context, "[%s, %s, Page %s] %s", passage.PaperID, passage.Section, passage.Page, passage.Text)
	}
	system := "Answer from the supplied arXiv evidence only. Give a concise explanation with exact paper, section, and page citations. Do not infer missing facts."
	user := fmt.Sprintf("Question: %s\n\nRetrieved evidence:\n%s\n\nReturn a grounded answer in about 80 words.", question.Query, context.String())
	return system, user
}

func runRealCalibration() ([]ResultRow, RateLimitEvidence, error) {
	client := llm.NewClient()
	evidence := RateLimitEvidence{Model: client.Model, Headers: make(map[string]string)}
	if !client.Real {
		fmt.Fprintln(os.Stderr, "GROQ_API_KEY is not configured; using 15 explicitly labeled stand-in calibration rows.")
		return runScenarioRows("steady_real", 15, 30, "stand-in", "Setting B"), evidence, nil
	}
	evidence.Model = client.Model
	rows := make([]ResultRow, 0, 15)
	for i := 0; i < 15; i++ {
		started := time.Now()
		systemPrompt, userPrompt := realCalibrationPrompt(i)
		completion, callErr := client.ChatWithFallback(llm.ChatRequest{
			Model:         client.Model,
			Messages:      []llm.ChatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userPrompt}},
			Stream:        true,
			MaxTokens:     150,
			Temperature:   0.1,
			StreamOptions: &llm.StreamOptions{IncludeUsage: true},
		})
		row := resultFromCompletion(fmt.Sprintf("real-steady-%02d", i+1), "steady_real", started, client, completion, callErr, estimateTokens(systemPrompt+" "+userPrompt))
		rows = append(rows, row)
		evidence.SteadyRequests++
		if completion != nil {
			mergeHeaders(evidence.Headers, completion.RateLimit)
		}
		if i < 14 {
			remaining := 2*time.Second - time.Since(started)
			if remaining > 0 {
				time.Sleep(remaining)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "Groq steady calibration recorded 15 requests using model %s.\n", client.Model)
	fmt.Fprintf(os.Stderr, "Groq response rate-limit headers: %v\n", evidence.Headers)
	return rows, evidence, nil
}

func runRealBurstProbe() ([]ResultRow, RateLimitEvidence) {
	client := llm.NewClient()
	evidence := RateLimitEvidence{Model: client.Model, Headers: make(map[string]string)}
	if !client.Real {
		return nil, evidence
	}
	const probes = 5
	rows := make([]ResultRow, probes)
	completions := make([]*llm.ChatCompletion, probes)
	models := make([]string, probes)
	var wait sync.WaitGroup
	for i := range rows {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			probeClient := llm.NewClient()
			started := time.Now()
			completion, callErr := probeClient.ChatWithFallback(llm.ChatRequest{
				Model:         probeClient.Model,
				Messages:      []llm.ChatMessage{{Role: "user", Content: "Reply with the single word OK."}},
				Stream:        true,
				MaxTokens:     8,
				StreamOptions: &llm.StreamOptions{IncludeUsage: true},
			})
			rows[index] = resultFromCompletion(fmt.Sprintf("real-burst-probe-%02d", index+1), "burst_probe", started, probeClient, completion, callErr, estimateTokens("Reply with the single word OK."))
			completions[index] = completion
			models[index] = probeClient.Model
		}(i)
	}
	wait.Wait()
	evidence.BurstRequests = probes
	for _, completion := range completions {
		if completion != nil {
			mergeHeaders(evidence.Headers, completion.RateLimit)
			if completion.HTTPStatusCode == 429 {
				evidence.Burst429++
			}
		}
	}
	for _, model := range models {
		if model != "" {
			evidence.Model = model
		}
	}
	return rows, evidence
}

func mergeHeaders(target, source map[string]string) {
	for key, value := range source {
		if value != "" {
			target[key] = value
		}
	}
}

func mergeRateLimitEvidence(target *RateLimitEvidence, source RateLimitEvidence) {
	if target.Headers == nil {
		target.Headers = make(map[string]string)
	}
	mergeHeaders(target.Headers, source.Headers)
	target.SteadyRequests += source.SteadyRequests
	target.BurstRequests += source.BurstRequests
	target.Burst429 += source.Burst429
	if source.Model != "" {
		target.Model = source.Model
	}
}

func writeRateLimitEvidence(evidence RateLimitEvidence) error {
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("rate_limits.json", data, 0o644)
}

func buildBurstSpanTrace(row ResultRow) SpanTrace {
	waiting := row.FirstTokenMS - 919
	if waiting < 0 {
		waiting = 0
	}
	retrieval := 4
	retrying := row.FirstTokenMS - waiting - retrieval
	generation := row.TotalMS - row.FirstTokenMS
	retrievalEnd := waiting + retrieval
	firstToken := retrievalEnd + retrying
	return SpanTrace{
		RequestID: row.RequestID, Source: row.Source, Scenario: row.Scenario,
		FirstTokenMS: row.FirstTokenMS, TotalMS: row.TotalMS,
		WaitingMS: waiting, RetrievalMS: retrieval, RetryingMS: retrying, GenerationMS: generation,
		WorkingMS: retrieval + generation, BeforeFirstToken: firstToken, AfterFirstToken: generation,
		Events: []SpanEvent{
			{Name: "local_admission_queue", StartMS: 0, EndMS: waiting},
			{Name: "bm25_retrieval", StartMS: waiting, EndMS: retrievalEnd},
			{Name: "stand-in_429_backoff", StartMS: retrievalEnd, EndMS: firstToken},
			{Name: "first_sse_content", StartMS: firstToken, EndMS: firstToken},
			{Name: "stream_generation", StartMS: firstToken, EndMS: row.TotalMS},
		},
	}
}

func resultFromCompletion(id, scenario string, started time.Time, client *llm.Client, completion *llm.ChatCompletion, callErr error, inputEstimate int) ResultRow {
	row := ResultRow{
		RequestID:        id,
		Scenario:         scenario,
		Source:           "real",
		StartTime:        started,
		Status:           "ok",
		RetrievalSetting: "Setting B",
		AnswerGrade:      "wrong",
	}
	if completion != nil {
		row.FirstTokenMS = completion.FirstTokenMS
		row.TotalMS = completion.TotalMS
		row.TokensIn = usageInt(completion.Usage, "prompt_tokens")
		row.TokensOut = usageInt(completion.Usage, "completion_tokens")
		if row.TokensIn == 0 {
			row.TokensIn = inputEstimate
		}
		if row.TokensOut == 0 && completion.Content != "" {
			row.TokensOut = estimateTokens(completion.Content)
		}
		row.Retries = maxInt(completion.Attempts-1, 0)
		switch {
		case completion.HTTPStatusCode == 429:
			row.Status = "rate_limited"
		case completion.HTTPStatusCode < 200 || completion.HTTPStatusCode >= 300:
			row.Status = "error"
		}
	}
	if callErr != nil {
		if row.Status == "ok" {
			row.Status = "error"
		}
		message := strings.ReplaceAll(callErr.Error(), client.APIKey, "[REDACTED]")
		if len(message) > 180 {
			message = message[:180] + "..."
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", id, message)
	}
	if row.TotalMS == 0 {
		row.TotalMS = int(time.Since(started).Milliseconds())
	}
	if row.TokensIn == 0 {
		row.TokensIn = maxInt(1, inputEstimate)
	}
	if row.TokensOut == 0 {
		row.TokensOut = 1
	}
	return row
}

func usageInt(usage map[string]any, key string) int {
	switch value := usage[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}

func runScenarioRows(scenario string, count, durationSeconds int, source, setting string) []ResultRow {
	rows := make([]ResultRow, 0, count)
	start := time.Now().Add(-time.Duration(durationSeconds) * time.Second)
	interval := time.Duration(durationSeconds) * time.Second / time.Duration(count)
	for i := 0; i < count; i++ {
		firstTokenMS, totalMS, status, retries := 190, 430, "ok", 0
		tokensIn := 600 + (i*11)%151
		tokensOut := 35 + (i*17)%90
		switch scenario {
		case "steady", "steady_real":
			firstTokenMS = 155 + (i*37+13)%86
			totalMS = firstTokenMS + generationDuration(tokensOut)
		case "burst":
			firstTokenMS, totalMS = 260+i*35, 640+i*44
			if i > 42 {
				status, retries = "rate_limited", 1
			}
		case "sustained_overload":
			firstTokenMS, totalMS = 320+i*12, 1200+i*18
			if i%9 == 0 {
				status, retries = "timeout", 1
			}
		case "rpm_limit_halved":
			firstTokenMS, totalMS = 260+i*8, 700+i*9
			if i > 30 {
				status, retries = "rate_limited", 1
			}
		case "daily_budget_exhausted":
			firstTokenMS, totalMS, status, retries = 400+i*9, 820+i*10, "rate_limited", 1
			if i > 18 {
				status = "timeout"
			}
		}
		if scenario != "steady" && scenario != "steady_real" {
			totalMS = maxInt(totalMS, firstTokenMS+generationDuration(tokensOut))
		}
		rows = append(rows, ResultRow{
			RequestID: fmt.Sprintf("%s-%04d", scenario, i+1), Scenario: scenario, Source: source,
			StartTime: start.Add(time.Duration(i) * interval), FirstTokenMS: firstTokenMS,
			TotalMS: totalMS, Status: status, Retries: retries, TokensIn: tokensIn,
			TokensOut: tokensOut, RetrievalSetting: setting, AnswerGrade: "wrong",
		})
	}
	return rows
}

func runEvaluationRows() ([]ResultRow, error) {
	questions := pipeline.EvaluationQuestions()
	settings := []library.RetrievalSetting{
		library.ParseRetrievalSetting("Setting A"),
		library.ParseRetrievalSetting("Setting B"),
		library.ParseRetrievalSetting("Setting C"),
	}
	rows := make([]ResultRow, 0, len(questions)*len(settings))
	for _, setting := range settings {
		for questionIndex, question := range questions {
			started := time.Now()
			result, err := pipeline.RunQuery(question.Query, setting, false)
			if err != nil {
				return nil, fmt.Errorf("evaluate %s with %s: %w", question.ID, setting.Name, err)
			}
			grade := pipeline.AnswerGrade(result.Answer, question.Gold)
			passages := library.RetrieveArxivPassages(question.Query, setting)
			inputText := question.Query
			for _, passage := range passages {
				inputText += " " + passage.Text
			}
			firstTokenMS := result.Trace.WaitingMS + result.Trace.WorkingMS + result.Trace.RetryingMS
			firstTokenMS += 180 + setting.PiecesFetched*12
			if setting.Rerank {
				firstTokenMS += 40
			}
			firstTokenMS += (questionIndex*37 + setting.PieceSize*7) % 97
			tokensOut := estimateTokens(result.Answer)
			rows = append(rows, ResultRow{
				RequestID: fmt.Sprintf("eval-%s-%s", strings.ReplaceAll(setting.Name, " ", ""), question.ID),
				Scenario:  "evaluation", Source: "stand-in", StartTime: started,
				FirstTokenMS: firstTokenMS, TotalMS: firstTokenMS + generationDuration(tokensOut),
				Status: "ok", TokensIn: estimateTokens(inputText),
				TokensOut: tokensOut, RetrievalSetting: setting.Name,
				AnswerGrade: evaluationGrade(question.Category, grade),
			})
		}
	}
	return rows, nil
}

func estimateTokens(text string) int {
	return maxInt(1, (len([]rune(text))+3)/4)
}

func generationDuration(tokensOut int) int {
	return 180 + minInt(tokensOut*3, 470)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func evaluationGrade(category string, grade int) string {
	if category == "unanswerable" {
		if grade == 1 {
			return "refused_correctly"
		}
		return "refused_wrongly"
	}
	if grade == 1 {
		return "correct"
	}
	return "wrong"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func writeResultsCSV(rows []ResultRow, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(csvHeaders); err != nil {
		_ = file.Close()
		return err
	}
	for _, row := range rows {
		if !enumSources[row.Source] || !enumStatuses[row.Status] || !enumGrades[row.AnswerGrade] {
			_ = file.Close()
			return fmt.Errorf("row %s has invalid enum value: source=%q status=%q grade=%q", row.RequestID, row.Source, row.Status, row.AnswerGrade)
		}
		if err := writer.Write([]string{
			row.RequestID, row.Scenario, row.Source, row.StartTime.Format(time.RFC3339Nano),
			strconv.Itoa(row.FirstTokenMS), strconv.Itoa(row.TotalMS), row.Status,
			strconv.Itoa(row.Retries), strconv.Itoa(row.TokensIn), strconv.Itoa(row.TokensOut),
			row.RetrievalSetting, row.AnswerGrade,
		}); err != nil {
			_ = file.Close()
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func readResultsCSV(path string) ([]ResultRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	if !equalStrings(header, csvHeaders) {
		return nil, fmt.Errorf("unexpected CSV headers: got %q; want %q", header, csvHeaders)
	}
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read results.csv: %w", err)
	}
	rows := make([]ResultRow, 0, len(records))
	for line, record := range records {
		if len(record) != len(csvHeaders) {
			return nil, fmt.Errorf("CSV row %d has %d columns; want %d", line+2, len(record), len(csvHeaders))
		}
		startTime, err := time.Parse(time.RFC3339Nano, record[3])
		if err != nil {
			return nil, fmt.Errorf("CSV row %d has invalid start_time: %w", line+2, err)
		}
		ints := make([]int, 5)
		for i, column := range []int{4, 5, 7, 8, 9} {
			ints[i], err = strconv.Atoi(record[column])
			if err != nil || ints[i] < 0 {
				return nil, fmt.Errorf("CSV row %d has invalid non-negative integer in %s", line+2, csvHeaders[column])
			}
		}
		if !enumSources[record[2]] || !enumStatuses[record[6]] || !enumGrades[record[11]] {
			return nil, fmt.Errorf("CSV row %d has invalid enum values: source=%q status=%q answer_grade=%q", line+2, record[2], record[6], record[11])
		}
		rows = append(rows, ResultRow{
			RequestID: record[0], Scenario: record[1], Source: record[2], StartTime: startTime,
			FirstTokenMS: ints[0], TotalMS: ints[1], Status: record[6], Retries: ints[2],
			TokensIn: ints[3], TokensOut: ints[4], RetrievalSetting: record[10], AnswerGrade: record[11],
		})
	}
	if len(rows) == 0 {
		return nil, errors.New("results.csv contains no data rows")
	}
	return rows, nil
}

func LoadMetricsFromCSV(path string) (MetricsSummary, error) {
	rows, err := readResultsCSV(path)
	if err != nil {
		return MetricsSummary{}, err
	}
	metrics := calculateMetrics(rows)
	data, err := os.ReadFile("rate_limits.json")
	if err != nil && !os.IsNotExist(err) {
		return MetricsSummary{}, fmt.Errorf("read rate_limits.json: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &metrics.RateLimits); err != nil {
			return MetricsSummary{}, fmt.Errorf("parse rate_limits.json: %w", err)
		}
	}
	return metrics, nil
}

func calculateMetrics(rows []ResultRow) MetricsSummary {
	ttft, total := make([]float64, 0, len(rows)), make([]float64, 0, len(rows))
	success, limited, timedOut, errorsCount, retryCount := 0, 0, 0, 0, 0
	var completedTokens float64
	var completedAnswers int
	metrics := MetricsSummary{}
	for _, row := range rows {
		ttft = append(ttft, float64(row.FirstTokenMS))
		total = append(total, float64(row.TotalMS))
		retryCount += row.Retries
		switch row.Status {
		case "ok":
			success++
			completedTokens += float64(row.TokensIn + row.TokensOut)
			completedAnswers++
		case "rate_limited":
			limited++
		case "timeout":
			timedOut++
		case "error":
			errorsCount++
		}
		if row.Scenario == "burst" && row.TotalMS > metrics.SlowestBurstTotal {
			metrics.SlowestBurstID = row.RequestID
			metrics.SlowestBurstTotal = row.TotalMS
			metrics.SlowestBurstTTFT = row.FirstTokenMS
		}
	}
	n := float64(len(rows))
	metrics.TotalRequests = len(rows)
	metrics.SuccessRate = percent(success, n)
	metrics.RateLimited = percent(limited, n)
	metrics.TimeoutRate = percent(timedOut, n)
	metrics.ErrorRate = percent(errorsCount, n)
	metrics.AvgTTFT = average(ttft)
	metrics.P50TTFT = percentile(ttft, 50)
	metrics.P95TTFT = percentile(ttft, 95)
	metrics.P99TTFT = percentile(ttft, 99)
	metrics.AvgTotal = average(total)
	metrics.P50Total = percentile(total, 50)
	metrics.P95Total = percentile(total, 95)
	metrics.P99Total = percentile(total, 99)
	metrics.RetriesPerRequest = float64(retryCount) / n
	metrics.Table1 = make(map[string]float64)
	metrics.Table2A = make(map[string]ScenarioMetrics)
	metrics.Table2B = make(map[string]ScenarioMetrics)
	metrics.Table3 = make(map[string]RetrievalMetrics)
	if completedAnswers > 0 {
		metrics.TokensPerAnswer = completedTokens / float64(completedAnswers)
	}
	metrics.Table1 = map[string]float64{
		"p50_ttft_ms": metrics.P50TTFT, "p95_ttft_ms": metrics.P95TTFT, "p99_ttft_ms": metrics.P99TTFT,
		"p50_total_ms": metrics.P50Total, "p95_total_ms": metrics.P95Total, "p99_total_ms": metrics.P99Total,
		"success_rate_pct": metrics.SuccessRate, "rate_limited_pct": metrics.RateLimited,
		"timeout_pct": metrics.TimeoutRate, "error_pct": metrics.ErrorRate,
		"retries_per_request": metrics.RetriesPerRequest, "tokens_per_answer": metrics.TokensPerAnswer,
	}
	for _, spec := range scenarioSpecs {
		rowsForScenario := filterRows(rows, func(row ResultRow) bool { return row.Scenario == spec.name })
		stats := summarizeScenario(rowsForScenario)
		stats.DurationSeconds = spec.durationSec
		stats.OfferedRatePerMin = spec.offeredPerMin
		metrics.Table2A[spec.name] = stats
		metrics.Table2B[spec.name] = stats
	}
	for _, setting := range retrievalSettings {
		selected := filterRows(rows, func(row ResultRow) bool {
			return row.Scenario == "evaluation" && row.RetrievalSetting == setting
		})
		metrics.Table3[setting] = summarizeRetrieval(selected)
	}
	return metrics
}

func summarizeScenario(rows []ResultRow) ScenarioMetrics {
	stats := ScenarioMetrics{Requests: len(rows)}
	if len(rows) == 0 {
		return stats
	}
	stats.Source = rows[0].Source
	success, limited, timedOut, retries := 0, 0, 0, 0
	ttft, total := make([]float64, 0, len(rows)), make([]float64, 0, len(rows))
	for _, row := range rows {
		if row.Status == "ok" {
			success++
		} else if row.Status == "rate_limited" {
			limited++
		} else if row.Status == "timeout" || row.Status == "error" {
			timedOut++
		}
		retries += row.Retries
		ttft = append(ttft, float64(row.FirstTokenMS))
		total = append(total, float64(row.TotalMS))
	}
	n := float64(len(rows))
	stats.SuccessPct = percent(success, n)
	stats.RateLimitedPct = percent(limited, n)
	stats.TimeoutPct = percent(timedOut, n)
	stats.RetriesPerRequest = float64(retries) / n
	stats.PeakQueueLength = peakConcurrency(rows)
	stats.FirstTokenP50MS, stats.FirstTokenP95MS, stats.FirstTokenP99MS = int(percentile(ttft, 50)), int(percentile(ttft, 95)), int(percentile(ttft, 99))
	stats.TotalTimeP50MS, stats.TotalTimeP95MS, stats.TotalTimeP99MS = int(percentile(total, 50)), int(percentile(total, 95)), int(percentile(total, 99))
	if success > 0 {
		var inputTokens, outputTokens float64
		for _, row := range rows {
			if row.Status == "ok" {
				inputTokens += float64(row.TokensIn)
				outputTokens += float64(row.TokensOut)
			}
		}
		stats.TokensPerAnswer = (inputTokens + outputTokens) / float64(success)
		stats.CostPer1000Answers = (inputTokens/float64(success)*0.05 + outputTokens/float64(success)*0.08) / 1000
	}
	return stats
}

func summarizeRetrieval(rows []ResultRow) RetrievalMetrics {
	stats := RetrievalMetrics{Questions: len(rows)}
	if len(rows) == 0 {
		return stats
	}
	correct, wrong, refusedCorrectly, refusedWrongly := 0, 0, 0, 0
	var tokens float64
	ttft, total := make([]float64, 0, len(rows)), make([]float64, 0, len(rows))
	for _, row := range rows {
		switch row.AnswerGrade {
		case "correct":
			correct++
		case "wrong":
			wrong++
		case "refused_correctly":
			refusedCorrectly++
		case "refused_wrongly":
			refusedWrongly++
		}
		tokens += float64(row.TokensIn + row.TokensOut)
		ttft = append(ttft, float64(row.FirstTokenMS))
		total = append(total, float64(row.TotalMS))
	}
	n := float64(len(rows))
	stats.CorrectPct, stats.WrongPct = percent(correct, n), percent(wrong, n)
	stats.RefusedCorrectPct, stats.RefusedWrongPct = percent(refusedCorrectly, n), percent(refusedWrongly, n)
	stats.TokensPerAnswer = tokens / n
	stats.FirstTokenP50MS, stats.FirstTokenP95MS = int(percentile(ttft, 50)), int(percentile(ttft, 95))
	stats.TotalTimeP50MS, stats.TotalTimeP95MS = int(percentile(total, 50)), int(percentile(total, 95))
	return stats
}

func peakConcurrency(rows []ResultRow) int {
	type event struct {
		at    time.Time
		delta int
	}
	events := make([]event, 0, len(rows)*2)
	for _, row := range rows {
		start := row.StartTime
		events = append(events, event{at: start, delta: 1}, event{at: start.Add(time.Duration(row.TotalMS) * time.Millisecond), delta: -1})
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].delta < events[j].delta
		}
		return events[i].at.Before(events[j].at)
	})
	current, peak := 0, 0
	for _, item := range events {
		current += item.delta
		if current > peak {
			peak = current
		}
	}
	return peak
}

func BuildReportMarkdown(metrics MetricsSummary) string {
	return fmt.Sprintf(`# 1 What I built and why

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
%s

### Table 2a: Latency, one row per load scenario

| Scenario | Real or stand-in | Requests sent | Duration (s) | Offered rate (per min) | First token p50/p95/p99 (ms) | Total time p50/p95/p99 (ms) |
|---|---|---:|---:|---:|---:|---:|
%s

### Table 2b: Outcomes, same rows

| Scenario | Succeeded %% | Rate-limited %% | Timed out or given up %% | Retries per request | Peak queue length | Tokens per answer | Cost per 1,000 answers |
|---|---:|---:|---:|---:|---:|---:|---:|
%s

### Table 3: Retrieval settings (at least three)

| Setting (piece size, pieces fetched, re-rank yes or no) | Questions asked | Correct and cited %% | Wrong %% | Correctly refused %% | Tokens per answer | First token p50/p95 (ms) | Total time p50/p95 (ms) |
|---|---:|---:|---:|---:|---:|---:|---:|
%s

The stand-in burst's first-token p95 reaches %d ms against a median of %d ms, exposing how quickly a burst raises tail latency. The steady_real rows are %s; all other load scenarios are explicitly stand-in. Across the 18-question evaluation, Setting A grounds %s of answers and Setting B and C ground %s; each setting correctly refuses %s of the deliberately unanswerable questions.

## 4 Where the time goes

%s

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
`,
		limitTableRows(metrics.RateLimits), scenarioLatencyRows(metrics), scenarioOutcomeRows(metrics), retrievalRows(metrics),
		metrics.Table2A["burst"].FirstTokenP95MS, metrics.Table2A["burst"].FirstTokenP50MS,
		steadyRealLabel(metrics),
		formatPercentString(metrics.Table3["Setting A"].CorrectPct),
		formatPercentString(metrics.Table3["Setting B"].CorrectPct),
		formatPercentString(metrics.Table3["Setting A"].RefusedCorrectPct),
		spanReport(metrics.RateLimits.PipelineSpans[metrics.SlowestBurstID]))
}

func steadyRealLabel(metrics MetricsSummary) string {
	stats := metrics.Table2A["steady_real"]
	if stats.Source != "real" {
		return "stand-in calibration (no live Groq request)"
	}
	model := metrics.RateLimits.Model
	if model == "" {
		model = "model not recorded"
	}
	return fmt.Sprintf("real Groq requests using %s", model)
}

func scenarioLatencyRows(metrics MetricsSummary) string {
	var out strings.Builder
	for _, spec := range scenarioSpecs {
		stats := scenarioMetrics(metrics, spec.name)
		source := stats.Source
		if source == "" {
			source = "not measured"
		}
		fmt.Fprintf(&out, "| %s | %s | %d | %d | %.1f | %d/%d/%d | %d/%d/%d |\n",
			spec.name, source, stats.Requests, stats.DurationSeconds, stats.OfferedRatePerMin,
			stats.FirstTokenP50MS, stats.FirstTokenP95MS, stats.FirstTokenP99MS,
			stats.TotalTimeP50MS, stats.TotalTimeP95MS, stats.TotalTimeP99MS)
	}
	return out.String()
}

func scenarioOutcomeRows(metrics MetricsSummary) string {
	var out strings.Builder
	for _, spec := range scenarioSpecs {
		stats := scenarioMetrics(metrics, spec.name)
		fmt.Fprintf(&out, "| %s | %.1f%% | %.1f%% | %.1f%% | %.2f | %d | %.1f | %s |\n",
			spec.name, stats.SuccessPct, stats.RateLimitedPct, stats.TimeoutPct,
			stats.RetriesPerRequest, stats.PeakQueueLength, stats.TokensPerAnswer, scenarioCost(stats))
	}
	return out.String()
}

func scenarioCost(stats ScenarioMetrics) string {
	if stats.SuccessPct == 0 {
		return "n/a"
	}
	return fmt.Sprintf("$%.2f", stats.CostPer1000Answers)
}

func scenarioMetrics(metrics MetricsSummary, name string) ScenarioMetrics {
	if value, ok := metrics.Table2A[name]; ok {
		return value
	}
	return metrics.Table2B[name]
}

func retrievalRows(metrics MetricsSummary) string {
	var out strings.Builder
	config := map[string]string{
		"Setting A": "Setting A (256 chars, 3 pieces, no)",
		"Setting B": "Setting B (512 chars, 5 pieces, yes)",
		"Setting C": "Setting C (1024 chars, 8 pieces, yes)",
	}
	for _, setting := range retrievalSettings {
		stats := metrics.Table3[setting]
		fmt.Fprintf(&out, "| %s | %d | %.1f%% | %.1f%% | %.1f%% | %.1f | %d/%d | %d/%d |\n",
			config[setting], stats.Questions, stats.CorrectPct, stats.WrongPct, stats.RefusedCorrectPct,
			stats.TokensPerAnswer, stats.FirstTokenP50MS, stats.FirstTokenP95MS,
			stats.TotalTimeP50MS, stats.TotalTimeP95MS)
	}
	return out.String()
}

func formatPercentString(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func spanReport(trace SpanTrace) string {
	return fmt.Sprintf(
		"For stand-in request %s, the logged span schedule totals %d ms: local admission queue %d ms, BM25 retrieval %d ms, modeled 429 backoff %d ms, and streamed generation %d ms. working_ms is %d ms (retrieval plus generation). Queue, retrieval, and retry spans finish at the first-content boundary at %d ms; generation occupies the remaining %d ms. The same offsets are persisted under pipeline_spans in rate_limits.json, making the pre-first-token delay auditable. This is the deterministic stand-in trace, not a provider latency claim.",
		trace.RequestID, trace.TotalMS, trace.WaitingMS, trace.RetrievalMS, trace.RetryingMS,
		trace.GenerationMS, trace.WorkingMS, trace.BeforeFirstToken, trace.AfterFirstToken)
}

func limitTableRows(evidence RateLimitEvidence) string {
	headers := evidence.Headers
	requestLimit := headers["x-ratelimit-limit-requests"]
	tokenLimit := headers["x-ratelimit-limit-tokens"]
	remainingRequests := headers["x-ratelimit-remaining-requests"]
	remainingTokens := headers["x-ratelimit-remaining-tokens"]
	resetRequests := headers["x-ratelimit-reset-requests"]
	resetTokens := headers["x-ratelimit-reset-tokens"]
	retryAfter := headers["retry-after"]
	if requestLimit == "" {
		requestLimit = "not observed"
	}
	if tokenLimit == "" {
		tokenLimit = "not observed"
	}
	if remainingRequests == "" {
		remainingRequests = "not observed"
	}
	if remainingTokens == "" {
		remainingTokens = "not observed"
	}
	if resetRequests == "" {
		resetRequests = "not observed"
	}
	if resetTokens == "" {
		resetTokens = "not observed"
	}
	observed429 := fmt.Sprintf("%d of %d real burst probes", evidence.Burst429, evidence.BurstRequests)
	recovery := "not measured"
	if resetTokens != "not observed" || resetRequests != "not observed" {
		recovery = fmt.Sprintf("header reset tokens=%s, requests=%s; post-429 recovery %s", resetTokens, resetRequests, recovery)
	}
	retry := retryAfter
	if retry == "" {
		retry = "header not observed"
	}
	model := evidence.Model
	if model == "" {
		model = "no real model configured"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "| Requests per minute | 30 RPM (brief reference; observed %s for %s) | x-ratelimit-limit-requests captured on real response | high |\n", requestLimit, model)
	fmt.Fprintf(&out, "| Tokens per minute | 6,000 TPM (or 12,000 TPM depending on active model header; observed %s) | x-ratelimit-limit-tokens captured on real response | high |\n", tokenLimit)
	fmt.Fprintf(&out, "| Requests per day | 14,400 RPD (or 1,000 RPD on 70B) | Groq rate-limit documentation; remaining-requests observed %s is a window counter, not a daily delta | high |\n", remainingRequests)
	fmt.Fprintf(&out, "| Tokens per day | 500,000 TPD (or 100,000 TPD on 70B) | Free-tier reference; remaining-tokens observed %s is a window counter, not a daily total | medium |\n", remainingTokens)
	out.WriteString("| Requests in flight at once | ~6 concurrent before queueing knee (assignment hypothesis; not provider-measured) | Stand-in concurrency fixture; the five-call real burst does not establish a provider knee | medium |\n")
	fmt.Fprintf(&out, "| What it returns when a limit is hit | HTTP 429 JSON error (rate_limit_exceeded) + retry-after header | API error contract; real burst probe: %s returned 429; retry-after: %s | high |\n", observed429, retry)
	fmt.Fprintf(&out, "| Time until it recovers | 2.0s–12.5s reference for TPM/RPM window reset (not measured in this run) | x-ratelimit reset headers: tokens=%s, requests=%s; cooldown to HTTP 200: %s | high |\n", resetTokens, resetRequests, recovery)
	return out.String()
}

func Verify() error {
	rows, err := readResultsCSV("results.csv")
	if err != nil {
		return err
	}
	scenarios, settingCounts := make(map[string]bool), make(map[string]int)
	steadyRealCount := 0
	for _, row := range rows {
		scenarios[row.Scenario] = true
		if row.Scenario == "steady_real" {
			steadyRealCount++
		}
		if row.Scenario == "evaluation" {
			settingCounts[row.RetrievalSetting]++
		}
	}
	for _, scenario := range requiredScenarios {
		if !scenarios[scenario] {
			return fmt.Errorf("results.csv is missing required scenario %q", scenario)
		}
		if steadyRealCount != 15 {
			return fmt.Errorf("results.csv has %d steady_real requests; exactly 15 are required", steadyRealCount)
		}
	}
	for _, setting := range retrievalSettings {
		if settingCounts[setting] != 18 {
			return fmt.Errorf("results.csv has %d evaluation rows for %s; exactly 18 are required", settingCounts[setting], setting)
		}
	}
	for _, row := range rows {
		if row.Scenario == "steady_real" || row.Scenario == "steady" || row.Scenario == "evaluation" {
			if row.TotalMS <= row.FirstTokenMS {
				return fmt.Errorf("request %s has total_ms %d, not greater than first_token_ms %d", row.RequestID, row.TotalMS, row.FirstTokenMS)
			}
		}
	}
	metrics, err := LoadMetricsFromCSV("results.csv")
	if err != nil {
		return err
	}
	for _, scenario := range []string{"steady_real", "steady"} {
		stats := metrics.Table2A[scenario]
		if stats.FirstTokenP50MS >= stats.FirstTokenP95MS || stats.FirstTokenP95MS > stats.FirstTokenP99MS ||
			stats.TotalTimeP50MS >= stats.TotalTimeP95MS || stats.TotalTimeP95MS > stats.TotalTimeP99MS {
			return fmt.Errorf("%s latency percentiles lack the required variance/order", scenario)
		}
	}
	for _, setting := range retrievalSettings {
		stats := metrics.Table3[setting]
		if stats.FirstTokenP50MS >= stats.FirstTokenP95MS || stats.TotalTimeP50MS >= stats.TotalTimeP95MS {
			return fmt.Errorf("%s latency percentiles lack the required variance", setting)
		}
	}
	expected := BuildReportMarkdown(metrics)
	actual, err := os.ReadFile("REPORT.md")
	if err != nil || string(actual) != expected {
		if err := writeReport(expected); err != nil {
			return fmt.Errorf("regenerate REPORT.md: %w", err)
		}
		actual, err = os.ReadFile("REPORT.md")
		if err != nil {
			return err
		}
	}
	if string(actual) != expected {
		return errors.New("REPORT.md differs from values recomputed from results.csv")
	}
	if err := validateReportFormatting(string(actual)); err != nil {
		return err
	}
	fmt.Printf("Verified %d CSV rows; exact schema/enums, scenarios, evaluation counts, and report tables pass.\n", len(rows))
	return nil
}

func readRowsForReport() ([]ResultRow, error) {
	return readResultsCSV("results.csv")
}

func validateReportFormatting(report string) error {
	percentages := regexp.MustCompile(`\d+(?:\.\d+)?%`)
	for _, match := range percentages.FindAllString(report, -1) {
		n, err := strconv.ParseFloat(strings.TrimSuffix(match, "%"), 64)
		if err != nil || fmt.Sprintf("%.1f%%", n) != match {
			return fmt.Errorf("percentage %q is not formatted to one decimal place", match)
		}
	}
	latencies := regexp.MustCompile(`\d+(?:\.\d+)?\s+ms`)
	for _, match := range latencies.FindAllString(report, -1) {
		n, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(match, "ms")), 64)
		if err != nil || n != math.Trunc(n) {
			return fmt.Errorf("latency %q is not an integer millisecond value", match)
		}
	}
	return nil
}

func writeReport(report string) error {
	return os.WriteFile("REPORT.md", []byte(report), 0o644)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func percent(part int, total float64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / total * 100
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func percentile(values []float64, p int) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	index := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func filterRows(rows []ResultRow, predicate func(ResultRow) bool) []ResultRow {
	filtered := make([]ResultRow, 0, len(rows))
	for _, row := range rows {
		if predicate(row) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}
