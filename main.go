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

	mux.HandleFunc("/api/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		question := strings.TrimSpace(r.FormValue("question"))
		if question == "" {
			question = strings.TrimSpace(r.FormValue("query"))
		}
		if question == "" {
			http.Error(w, "question is required", http.StatusBadRequest)
			return
		}
		settingName := r.FormValue("setting")
		if settingName == "" {
			settingName = "Balanced-Reranked"
		}
		setting := library.ParseRetrievalSetting(settingName)
		result, err := pipeline.RunQuery(question, setting, r.FormValue("mode") == "real")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, map[string]any{
			"status":    "ok",
			"question":  question,
			"answer":    result.Answer,
			"citations": result.Citations,
			"setting":   result.Setting,
			"trace": map[string]any{
				"waiting_ms":     result.Trace.WaitingMS,
				"working_ms":     result.Trace.WorkingMS,
				"retrying_ms":    result.Trace.RetryingMS,
				"retrieval_ms":   result.Trace.RetrievalMS,
				"synthesis_ms":   result.Trace.SynthesisMS,
				"first_token_ms": result.Trace.FirstTokenMS,
				"generation_ms":  result.Trace.GenerationMS,
				"total_ms":       result.Trace.TotalMS,
			},
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
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Scenario string `json:"scenario"`
			Count    int    `json:"count"`
		}
		if err := jsonDecoder(r.Body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.Scenario == "" {
			req.Scenario = "steady"
		}
		if req.Count <= 0 {
			req.Count = 10
		}
		output := llm.NewStandInEngine(30, 6000, 1200, 1200, 10)
		status, err := output.SimulateScenario(req.Scenario, req.Count)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		response := map[string]any{"scenario": req.Scenario, "count": req.Count, "status": status}
		if req.Scenario == "daily_budget_exhausted" {
			passages := library.RetrieveArxivPassages("What is Moshi's theoretical latency?", library.ParseRetrievalSetting("Setting B"))
			if len(passages) < 2 {
				http.Error(w, "daily-budget fallback requires two retrieved passages", http.StatusInternalServerError)
				return
			}
			excerpts := make([]map[string]string, 0, 2)
			for _, passage := range passages[:2] {
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
