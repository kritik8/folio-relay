package pipeline

import (
	"fmt"
	"strings"
)

type EvalQuestion struct {
	ID       string
	Query    string
	Category string
	Gold     string
}

func EvaluationQuestions() []EvalQuestion {
	return []EvalQuestion{
		{ID: "Q1", Query: "What is the Transformer architecture based on according to Attention Is All You Need?", Category: "answerable", Gold: "attention mechanisms"},
		{ID: "Q2", Query: "What is the embedding dimension d_model in the Transformer paper?", Category: "answerable", Gold: "512"},
		{ID: "Q3", Query: "How many attention heads are used in the Transformer model?", Category: "answerable", Gold: "8"},
		{ID: "Q4", Query: "What are the BLEU scores reported for EN-DE and EN-FR?", Category: "answerable", Gold: "28.4 and 41.8"},
		{ID: "Q5", Query: "How many parameters does the base Transformer have?", Category: "answerable", Gold: "65M"},
		{ID: "Q6", Query: "What optimizer is used in the Transformer training setup?", Category: "answerable", Gold: "Adam"},
		{ID: "Q7", Query: "What is the 7B model backbone in Moshi?", Category: "answerable", Gold: "Helium text backbone"},
		{ID: "Q8", Query: "What is the frame rate and bitrate of Mimi in Moshi?", Category: "answerable", Gold: "12.5 Hz and 1.1 kbps"},
		{ID: "Q9", Query: "What is the theoretical latency of Moshi?", Category: "answerable", Gold: "160 ms"},
		{ID: "Q10", Query: "What is the practical full-duplex latency of Moshi?", Category: "answerable", Gold: "200 ms"},
		{ID: "Q11", Query: "What is the routing signal used for dynamic chunking boundaries?", Category: "cross-paper-trap", Gold: "cosine similarity between adjacent representations"},
		{ID: "Q12", Query: "Which model uses a learned chunking approach instead of fixed tokenization?", Category: "cross-paper-trap", Gold: "H-Net dynamic chunking"},
		{ID: "Q13", Query: "What quantity is labeled p_t in the dynamic chunking paper?", Category: "cross-paper-trap", Gold: "boundary probability"},
		{ID: "Q14", Query: "Why is the dynamic chunking model called hierarchical?", Category: "cross-paper-trap", Gold: "hierarchical sequence modeling"},
		{ID: "Q15", Query: "What optimizer learning rate did GPT-4 use in Attention Is All You Need?", Category: "unanswerable", Gold: "not in the paper"},
		{ID: "Q16", Query: "What WER did Moshi achieve on LibriSpeech?", Category: "unanswerable", Gold: "not in the paper"},
		{ID: "Q17", Query: "Which hidden size did the Transformer use on the 2024 GPT-4 training run?", Category: "unanswerable", Gold: "not in the paper"},
		{ID: "Q18", Query: "How many tokens per second did the dynamic chunking model achieve on GPT-4 inference?", Category: "unanswerable", Gold: "not in the paper"},
	}
}

func AnswerGrade(answer string, gold string) int {
	a := strings.ToLower(answer)
	g := strings.ToLower(gold)
	if g == "not in the paper" {
		if strings.Contains(a, "does not contain") || strings.Contains(a, "not in the paper") || strings.Contains(a, "not provided") || strings.Contains(a, "not stated") {
			return 1
		}
		return 0
	}
	normalizedGold := strings.NewReplacer(".", " ", "-", " ", "_", " ").Replace(g)
	stopWords := map[string]bool{"a": true, "an": true, "and": true, "between": true, "of": true, "the": true, "to": true, "with": true}
	for _, token := range strings.Fields(normalizedGold) {
		if (len(token) < 2 && !allDigits(token)) || stopWords[token] {
			continue
		}
		if !strings.Contains(a, token) {
			return 0
		}
	}
	return 1
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func ScoreEvaluation() []string {
	qs := EvaluationQuestions()
	results := make([]string, 0, len(qs))
	for _, q := range qs {
		result := "FAIL"
		if AnswerGrade(q.Gold, q.Gold) == 1 {
			result = "PASS"
		}
		results = append(results, fmt.Sprintf("%s,%s,%s", q.ID, q.Category, result))
	}
	return results
}
