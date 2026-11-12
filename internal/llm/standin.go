package llm

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StandInEngine struct {
	mu         sync.Mutex
	RPM        int
	TPM        int
	RPD        int
	TPD        int
	InFlight   int
	DailyUsed  int
	queueDepth int
	rand       *rand.Rand
}

func NewStandInEngine(rpm, tpm, rpd, tpd, queueDepth int) *StandInEngine {
	if rpm <= 0 {
		rpm = 30
	}
	if tpm <= 0 {
		tpm = 6000
	}
	if rpd <= 0 {
		rpd = 1200
	}
	if tpd <= 0 {
		tpd = 120000
	}
	return &StandInEngine{
		RPM:        rpm,
		TPM:        tpm,
		RPD:        rpd,
		TPD:        tpd,
		queueDepth: queueDepth,
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *StandInEngine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	s.InFlight++
	s.queueDepth++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.InFlight--
		s.queueDepth--
		s.mu.Unlock()
	}()

	msg := "The Transformer avoids recurrence and relies solely on attention."
	words := strings.Fields(msg)
	for i, word := range words {
		if i > 0 {
			time.Sleep(90 * time.Millisecond)
		}
		_, _ = w.Write([]byte("event: token\ndata: {\"token\":\"" + escapeJSON(word) + "\"}\n\n"))
		flusher.Flush()
	}
}

func (s *StandInEngine) SimulateScenario(name string, count int) (map[string]any, error) {
	if count <= 0 {
		count = 1
	}
	accepted := 0
	rateLimited := 0
	timeouts := 0
	totalTTFT := 0
	totalLatency := 0
	queuePeak := 0
	for i := 0; i < count; i++ {
		s.mu.Lock()
		if s.InFlight > queuePeak {
			queuePeak = s.InFlight
		}
		s.mu.Unlock()
		if name == "burst" && i > 20 {
			rateLimited++
			continue
		}
		if name == "daily_budget_exhausted" && i > 12 {
			rateLimited++
			continue
		}
		if i%7 == 0 && name == "sustained_overload" {
			timeouts++
			continue
		}
		accepted++
		totalTTFT += 190 + i*10
		totalLatency += 420 + i*8
	}
	return map[string]any{
		"scenario":     name,
		"count":        count,
		"accepted":     accepted,
		"rate_limited": rateLimited,
		"timeouts":     timeouts,
		"avg_ttft_ms":  float64(totalTTFT) / float64(max(accepted, 1)),
		"avg_total_ms": float64(totalLatency) / float64(max(accepted, 1)),
		"queue_peak":   queuePeak,
	}, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func (s *StandInEngine) QueueDelay() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.InFlight <= 0 {
		return 0
	}
	// Modeled as log-like queue cost that increases sharply beyond 6 in-flight requests.
	base := 15 * (float64(s.InFlight) * float64(s.InFlight))
	return int(base / 8)
}

func (s *StandInEngine) AdmissionOK() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.InFlight < 8
}

func (s *StandInEngine) WithBudget() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DailyUsed < s.TPD
}

func (s *StandInEngine) StateSnapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"rpm":        s.RPM,
		"tpm":        s.TPM,
		"rpd":        s.RPD,
		"tpd":        s.TPD,
		"in_flight":  s.InFlight,
		"daily_used": s.DailyUsed,
	}
}

func (s *StandInEngine) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.StateSnapshot())
}

func RenderRutileStatus(scenario string, remainingRPM int, remainingTPM int, setting string) string {
	return fmt.Sprintf("scenario=%s rpm=%d tpm=%d setting=%s", scenario, remainingRPM, remainingTPM, setting)
}

func ParseRateLimitHeaders(headers map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range headers {
		out[k] = v
	}
	return out
}

func BuildFailureResponse(code int, reason string) map[string]any {
	return map[string]any{"status": code, "error": reason, "retryable": code == http.StatusTooManyRequests}
}

func NextBurstToken() string {
	vals := []string{"attention", "latency", "retrieval", "budget", "router", "chunking", "grounding"}
	return vals[int((time.Now().UnixNano()/997)%int64(len(vals)))]
}

func (s *StandInEngine) SimulateRequest() (string, int) {
	ttft := 180 + s.rand.Intn(220)
	if s.rand.Intn(100) < 5 {
		ttft += 1800 + s.rand.Intn(1400)
	}
	if s.InFlight > 6 {
		ttft += 15 * (s.InFlight * s.InFlight) / 6
	}
	response := "The model is grounded in the retrieved passages and cites the relevant paper sections."
	return response, ttft
}

func ScenarioStatusString(scenario string) string {
	switch scenario {
	case "steady":
		return "steady-state operation"
	case "burst":
		return "short burst with queue amplification"
	case "sustained_overload":
		return "stability under sustained overload"
	case "rpm_limit_halved":
		return "rpm limit halved"
	case "daily_budget_exhausted":
		return "daily budget exhausted"
	default:
		return scenario
	}
}

func ParseRequestDetail(raw string) map[string]string {
	result := map[string]string{}
	parts := strings.Split(raw, "&")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func FormatHeaderValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	default:
		return fmt.Sprintf("%v", v)
	}
}
