package library

import (
	"math"
	"sort"
	"strings"
)

// RetrievalSetting describes a retrieval configuration.
type RetrievalSetting struct {
	Name          string
	PieceSize     int
	PiecesFetched int
	Rerank        bool
}

func DefaultSetting() RetrievalSetting {
	return RetrievalSetting{Name: "Setting B", PieceSize: 512, PiecesFetched: 5, Rerank: true}
}

func ParseRetrievalSetting(name string) RetrievalSetting {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "a", "small-fast", "setting a":
		return RetrievalSetting{Name: "Setting A", PieceSize: 256, PiecesFetched: 3, Rerank: false}
	case "b", "balanced-reranked", "setting b":
		return DefaultSetting()
	case "c", "large-heavy", "setting c":
		return RetrievalSetting{Name: "Setting C", PieceSize: 1024, PiecesFetched: 8, Rerank: true}
	default:
		return DefaultSetting()
	}
}

func RetrieveArxivPassages(query string, setting RetrievalSetting) []Passage {
	if setting.PieceSize <= 0 {
		setting = DefaultSetting()
	}
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return nil
	}
	q := normalizeQuery(query)
	scored := make([]struct {
		p     Passage
		score float64
	}, 0)
	for _, group := range corpus {
		for _, p := range group {
			score := scorePassage(q, p)
			if score > 0 {
				scored = append(scored, struct {
					p     Passage
					score float64
				}{p: p, score: score})
			}
		}
	}
	if len(scored) == 0 {
		return nil
	}
	if setting.Rerank {
		sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	} else {
		sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	}
	limit := setting.PiecesFetched
	if limit <= 0 || limit > len(scored) {
		limit = len(scored)
	}
	out := make([]Passage, 0, limit)
	for i := 0; i < limit; i++ {
		passage := scored[i].p
		runes := []rune(passage.Text)
		if len(runes) > setting.PieceSize {
			passage.Text = string(runes[:setting.PieceSize])
		}
		out = append(out, passage)
	}
	return out
}

func normalizeQuery(q string) []string {
	q = strings.ToLower(q)
	q = strings.NewReplacer("?", " ", ".", " ", ",", " ", "/", " ", "-", " ", "_", " ", "\n", " ").Replace(q)
	parts := strings.Fields(q)
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "according": true, "based": true,
		"does": true, "for": true, "from": true, "have": true, "how": true, "in": true,
		"is": true, "it": true, "many": true, "of": true, "on": true, "paper": true,
		"the": true, "to": true, "used": true, "what": true, "which": true, "why": true,
	}
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 1 && !stopWords[p] {
			cleaned = append(cleaned, p)
		}
	}
	return cleaned
}

func scorePassage(query []string, p Passage) float64 {
	text := strings.ToLower(p.Text)
	title := strings.ToLower(p.Title)
	section := strings.ToLower(p.Section)
	score := 0.0
	for _, term := range query {
		if term == "" {
			continue
		}
		base := 1.0
		if strings.Contains(title, term) {
			base += 8.0
		}
		if strings.Contains(section, term) {
			base += 3.0
		}
		if strings.Contains(text, term) {
			base += 2.0
		}
		// term density approximates lexical strength without external libs
		count := strings.Count(text, term)
		if count > 0 {
			score += base * float64(count)
		}
	}
	return math.Max(0, score)
}
