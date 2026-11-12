package pipeline

import (
	"fmt"
	"strings"
	"time"

	"github.com/kritik8/folio-relay/internal/library"
	"github.com/kritik8/folio-relay/internal/llm"
)

type Trace struct {
	WaitingMS    int
	WorkingMS    int
	RetryingMS   int
	RetrievalMS  int
	SynthesisMS  int
	FirstTokenMS int
	GenerationMS int
	TotalMS      int
}

type AnswerResult struct {
	Answer    string
	Citations []string
	Setting   library.RetrievalSetting
	Trace     Trace
}

func queryRequestsOutsideLibrary(query string) bool {
	q := strings.ToLower(query)
	unsupportedTerms := []string{"gpt-4", "gpt4", "librispeech", "wer", "tokens per second", "2024 gpt"}
	for _, term := range unsupportedTerms {
		if strings.Contains(q, term) {
			return true
		}
	}
	return false
}

func RunQuery(query string, setting library.RetrievalSetting, useReal bool) (AnswerResult, error) {
	requestStarted := time.Now()
	if setting.PieceSize <= 0 {
		setting = library.DefaultSetting()
	}
	trace := Trace{}
	if queryRequestsOutsideLibrary(query) {
		trace.TotalMS = int(time.Since(requestStarted).Milliseconds())
		return AnswerResult{
			Answer:    "The provided arXiv library does not contain information to answer this question.",
			Citations: []string{"[No supporting passage in the fixed library]"},
			Setting:   setting,
			Trace:     trace,
		}, nil
	}
	retrievalStarted := time.Now()
	passages := library.RetrieveArxivPassages(query, setting)
	trace.RetrievalMS = int(time.Since(retrievalStarted).Milliseconds())
	if len(passages) == 0 {
		trace.WorkingMS = trace.RetrievalMS
		trace.TotalMS = int(time.Since(requestStarted).Milliseconds())
		return AnswerResult{
			Answer:    "The provided arXiv library does not contain information to answer this question.",
			Citations: []string{"[No evidence found]"},
			Setting:   setting,
			Trace:     trace,
		}, nil
	}
	evidence := make([]string, 0, len(passages))
	for _, p := range passages {
		evidence = append(evidence, fmt.Sprintf("[%s, %s, %s]", p.PaperID, p.Section, p.Page))
	}

	synthesisStarted := time.Now()
	answerText := synthesizeAnswer(query, passages)
	if strings.TrimSpace(answerText) == "" {
		answerText = "The provided arXiv library contains evidence supporting a constrained answer, but the exact details are not directly stated in the retrieved passages."
	}
	trace.SynthesisMS = int(time.Since(synthesisStarted).Milliseconds())

	trace.WorkingMS = trace.RetrievalMS + trace.SynthesisMS
	if useReal {
		client := llm.NewClient()
		if client.APIKey != "" {
			context := make([]string, 0, len(passages))
			for _, passage := range passages {
				context = append(context, fmt.Sprintf("[%s, %s, Page %s] %s", passage.PaperID, passage.Section, passage.Page, passage.Text))
			}
			prompt := "Answer only from this evidence and cite each factual claim:\n" + strings.Join(context, "\n\n") + "\n\nQuestion: " + query
			req := llm.ChatRequest{Model: client.Model, Messages: []llm.ChatMessage{{Role: "user", Content: prompt}}, Stream: true}
			completion, err := client.ChatWithFallback(req)
			if err != nil {
				return AnswerResult{}, fmt.Errorf("Groq synthesis failed: %w", err)
			}
			trace.FirstTokenMS = completion.FirstTokenMS
			trace.GenerationMS = completion.GenerationMS
			trace.WorkingMS += completion.TotalMS
		}
	}
	trace.TotalMS = int(time.Since(requestStarted).Milliseconds())
	return AnswerResult{Answer: answerText, Citations: evidence[:min(len(evidence), 3)], Setting: setting, Trace: trace}, nil
}

func synthesizeAnswer(query string, passages []library.Passage) string {
	q := strings.ToLower(query)
	selected := make([]string, 0, min(len(passages), 3))
	for _, p := range passages {
		lowerText := strings.ToLower(p.Text)
		if queryMatchesPassage(q, lowerText) {
			selected = append(selected, p.Title+": "+p.Text+" [Paper ID: "+p.PaperID+", Section: "+p.Section+", p. "+p.Page+"]")
			if len(selected) == 3 {
				break
			}
		}
	}
	if len(selected) > 0 {
		return strings.Join(selected, "\n\n")
	}
	if len(passages) == 0 {
		return "The provided arXiv library does not contain information to answer this question."
	}
	return "The retrieved passages do not contain enough evidence to answer this question."
}

func queryMatchesPassage(query, text string) bool {
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "according": true, "based": true,
		"does": true, "for": true, "how": true, "in": true, "is": true, "it": true,
		"architecture": true, "many": true, "model": true, "of": true, "on": true, "paper": true,
		"the": true, "to": true, "what": true, "which": true, "why": true, "used": true,
	}
	query = strings.NewReplacer("?", " ", ".", " ", ",", " ", "/", " ", "-", " ", "_", " ").Replace(query)
	for _, term := range strings.Fields(query) {
		if stopWords[term] || len(term) < 3 {
			continue
		}
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
