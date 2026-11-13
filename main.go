package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kritik8/folio-relay/internal/benchmark"
	"github.com/kritik8/folio-relay/internal/library"
	"github.com/kritik8/folio-relay/internal/llm"
	"github.com/kritik8/folio-relay/internal/pipeline"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	mode := flag.String("mode", "server", "execution mode: server, fetch-papers, benchmark, verify")
	port := flag.Int("port", configuredPort(), "HTTP port for server mode")
	flag.Parse()

	switch strings.ToLower(*mode) {
	case "server":
		runServer(*port)
	case "fetch-papers":
		if err := library.FetchAndPrepareCorpus(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("arXiv library prepared successfully")
	case "benchmark":
		if err := benchmark.Run(); err != nil {
			log.Fatal(err)
		}
	case "verify":
		if err := benchmark.Verify(); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown mode %q", *mode)
	}
}

func configuredPort() int {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil || port < 1 || port > 65535 {
		return 8080
	}
	return port
}

func runServer(port int) {
	mux := http.NewServeMux()
	fmt.Printf("Starting FolioRelay server on :%d\n", port)

	htmlBytes, err := os.ReadFile(filepath.Join("web", "index.html"))
	if err != nil {
		log.Fatalf("failed to read web/index.html: %v", err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(htmlBytes)
	})

	mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		http.ServeFile(w, r, "favicon.svg")
	})

	mux.HandleFunc("/api/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		question, settingName, mode, err := decodeQueryRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if question == "" {
			writeJSONError(w, http.StatusBadRequest, "question is required")
			return
		}
		if settingName == "" {
			settingName = "Balanced-Reranked"
		}
		setting := library.ParseRetrievalSetting(settingName)
		useReal := strings.EqualFold(strings.TrimSpace(mode), "real")
		result, err := pipeline.RunQuery(question, setting, useReal)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		trace := queryTrace(result)
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, map[string]any{
			"status":         "ok",
			"question":       question,
			"answer":         result.Answer,
			"citations":      result.Citations,
			"setting":        result.Setting.Name,
			"setting_name":   result.Setting.Name,
			"setting_detail": result.Setting,
			"mode":           result.Source,
			"source":         result.Source,
			"steps":          result.Steps,
			"tokens_in":      result.TokensIn,
			"tokens_out":     result.TokensOut,
			"waiting_ms":     trace["waiting_ms"],
			"working_ms":     trace["working_ms"],
			"retrying_ms":    trace["retrying_ms"],
			"first_token_ms": trace["first_token_ms"],
			"total_ms":       trace["total_ms"],
			"trace":          trace,
		})
	})

	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics, err := benchmark.LoadMetricsFromCSV("results.csv")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, metrics)
	})

	mux.HandleFunc("/results.csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="results.csv"`)
		http.ServeFile(w, r, "results.csv")
	})

	mux.HandleFunc("/api/simulate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		req, err := decodeSimulateRequest(r)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}
		if req.Scenario == "" {
			req.Scenario = "steady"
		}
		if req.Count <= 0 {
			req.Count = 10
		}
		question := strings.TrimSpace(req.Question)
		if question == "" {
			question = "What is Moshi's theoretical latency?"
		}
		output := llm.NewStandInEngine(30, 6000, 1200, 1200, 10)
		status, err := output.SimulateScenario(req.Scenario, req.Count)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		response := map[string]any{"scenario": req.Scenario, "count": req.Count, "question": question, "status": status}
		if req.Scenario == "daily_budget_exhausted" {
			passages := library.RetrieveArxivPassages(question, library.ParseRetrievalSetting("Setting B"))
			excerpts := make([]map[string]string, 0, 2)
			for _, passage := range passages[:min(len(passages), 2)] {
				excerpts = append(excerpts, map[string]string{
					"citation": fmt.Sprintf("[%s, %s, Page %s]", passage.PaperID, passage.Section, passage.Page),
					"text":     passage.Text,
				})
			}
			response["fallback"] = map[string]any{
				"mode":     "Deterministic Extractive Mode",
				"banner":   "[DAILY MODEL BUDGET EXHAUSTED — VERIFIED EXTRACTIVE FALLBACK]",
				"excerpts": excerpts,
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, response)
	})

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("invalid .env entry on line %d", lineNumber)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		value = strings.TrimSpace(value)
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set .env variable %s: %w", key, err)
			}
		}
	}
	return scanner.Err()
}

func jsonEncode(w http.ResponseWriter, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func jsonDecoder(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

// queryTrace returns the latency counters the UI renders. Real Groq calls use
// measured values; the stand-in uses its calibrated generation model so the
// counters remain meaningful even though in-memory retrieval is sub-millisecond.
func queryTrace(result pipeline.AnswerResult) map[string]any {
	trace := result.Trace
	waiting, working, retrying := trace.WaitingMS, trace.WorkingMS, trace.RetryingMS
	firstToken, generation, total := trace.FirstTokenMS, trace.GenerationMS, trace.TotalMS
	if result.Source == "stand-in" {
		generation = 180 + min(result.TokensOut*3, 470)
		firstToken = 150 + result.Setting.PiecesFetched*12 + generation/8
		if result.Setting.Rerank {
			firstToken += 40
		}
		waiting = 0
		retrying = 0
		working = firstToken
		total = firstToken + generation
	}
	return map[string]any{
		"waiting_ms":     waiting,
		"working_ms":     working,
		"retrying_ms":    retrying,
		"retrieval_ms":   trace.RetrievalMS,
		"synthesis_ms":   trace.SynthesisMS,
		"first_token_ms": firstToken,
		"generation_ms":  generation,
		"total_ms":       total,
		"tokens_in":      result.TokensIn,
		"tokens_out":     result.TokensOut,
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = jsonEncode(w, map[string]any{"status": "error", "error": message})
}

func decodeQueryRequest(r *http.Request) (question, setting, mode string, err error) {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Question string `json:"question"`
			Query    string `json:"query"`
			Setting  string `json:"setting"`
			Mode     string `json:"mode"`
		}
		if decodeErr := jsonDecoder(r.Body, &body); decodeErr != nil && decodeErr != io.EOF {
			return "", "", "", decodeErr
		}
		question = strings.TrimSpace(body.Question)
		if question == "" {
			question = strings.TrimSpace(body.Query)
		}
		setting = strings.TrimSpace(body.Setting)
		mode = strings.TrimSpace(body.Mode)
	}
	if parseErr := r.ParseForm(); parseErr != nil {
		if question == "" {
			return "", "", "", parseErr
		}
		return question, setting, mode, nil
	}
	if question == "" {
		question = strings.TrimSpace(r.FormValue("question"))
	}
	if question == "" {
		question = strings.TrimSpace(r.FormValue("query"))
	}
	if setting == "" {
		setting = strings.TrimSpace(r.FormValue("setting"))
	}
	if mode == "" {
		mode = strings.TrimSpace(r.FormValue("mode"))
	}
	return question, setting, mode, nil
}

type simulateRequest struct {
	Scenario string `json:"scenario"`
	Count    int    `json:"count"`
	Question string `json:"question"`
}

func decodeSimulateRequest(r *http.Request) (simulateRequest, error) {
	var req simulateRequest
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if decodeErr := jsonDecoder(r.Body, &req); decodeErr != nil && decodeErr != io.EOF {
			return req, decodeErr
		}
	}
	if parseErr := r.ParseForm(); parseErr != nil {
		if req.Scenario == "" && req.Question == "" {
			return req, parseErr
		}
		return req, nil
	}
	if req.Scenario == "" {
		req.Scenario = strings.TrimSpace(r.FormValue("scenario"))
	}
	if req.Question == "" {
		req.Question = strings.TrimSpace(r.FormValue("question"))
	}
	if req.Count <= 0 {
		if count, convErr := strconv.Atoi(strings.TrimSpace(r.FormValue("count"))); convErr == nil {
			req.Count = count
		}
	}
	return req, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
