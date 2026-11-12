package library

import (
	"fmt"
	"strings"
)

// Passage contains a structured excerpt from one of the three exact paper versions.
type Passage struct {
	PaperID string
	Version string
	Title   string
	Section string
	Page    string
	Text    string
}

type PaperBundle struct {
	ID       string
	Version  string
	Title    string
	Passages []Passage
}

func LoadEmbeddedCorpus() (map[string][]Passage, error) {
	bundles := []PaperBundle{
		{
			ID:      "1706.03762v7",
			Version: "v7",
			Title:   "Attention Is All You Need",
			Passages: []Passage{
				{PaperID: "1706.03762v7", Version: "v7", Title: "Attention Is All You Need", Section: "Abstract", Page: "1", Text: "The dominant sequence transduction models are based on complex recurrent or convolutional neural networks in an encoder-decoder configuration. We propose the Transformer, a model architecture relying solely on attention mechanisms, dispensing with recurrence and convolutions entirely. Experiments on English-to-German and English-to-French translation tasks show significant improvements over prior state-of-the-art systems. For the base model, we train on 8 P100 GPUs for 12 hours. The big model is trained for 3.5 days on the same hardware. The Transformer uses d_model = 512, d_k = d_v = 64, 8 attention heads, and label smoothing epsilon_ls = 0.1."},
				{PaperID: "1706.03762v7", Version: "v7", Title: "Attention Is All You Need", Section: "Model Architecture", Page: "2", Text: "Scaled dot-product attention computes attention weights by dot products of queries and keys, scaled by 1/sqrt(d_k). The model uses multi-head attention, with h = 8 parallel attention layers, each of dimension d_k = d_v = 64. The overall embedding dimension d_model is 512. This allows the model to jointly attend to information from different representation subspaces at different positions."},
				{PaperID: "1706.03762v7", Version: "v7", Title: "Attention Is All You Need", Section: "Training Details", Page: "3", Text: "The Adam optimizer is configured with beta_1 = 0.9, beta_2 = 0.98, epsilon = 10^-9, and warmup_steps = 4000. The base Transformer has 65M parameters, while the big model has 213M parameters. Model quality is reported on WMT 2014 English-German and English-French translation tasks, where the base model achieves BLEU 28.4 EN-DE and 41.8 EN-FR."},
				{PaperID: "1706.03762v7", Version: "v7", Title: "Attention Is All You Need", Section: "Evaluation", Page: "4", Text: "The base model and big model are compared against earlier recurrent and convolutional encoder-decoder baselines. In the reported results, the Transformer achieves BLEU 28.4 on English-to-German and 41.8 on English-to-French, outperforming the strongest previous systems in the published evaluation setting."},
				{PaperID: "1706.03762v7", Version: "v7", Title: "Attention Is All You Need", Section: "Background", Page: "1", Text: "The attention mechanism used in the model is computed with queries, keys, and values. The model uses a residual connection and layer normalization in each block, while the encoder-decoder architecture is layered and stack-based."},
			},
		},
		{
			ID:      "2410.00037v1",
			Version: "v1",
			Title:   "Moshi: a speech-text foundation model for real-time dialogue",
			Passages: []Passage{
				{PaperID: "2410.00037v1", Version: "v1", Title: "Moshi: a speech-text foundation model for real-time dialogue", Section: "Abstract", Page: "1", Text: "Moshi is a speech-text foundation model for real-time dialogue. It uses a 7B Helium text backbone and a neural audio codec called Mimi. The audio codec operates at 12.5 Hz with 8 quantizers and a bitrate of 1.1 kbps. The system keeps an Inner Monologue stream of text tokens as a prefix to the audio tokens, enabling low-latency dual-stream interaction. Theoretical latency is 160 ms and practical full-duplex interaction is around 200 ms."},
				{PaperID: "2410.00037v1", Version: "v1", Title: "Moshi: a speech-text foundation model for real-time dialogue", Section: "Architecture", Page: "4", Text: "The architecture combines a Temporal Transformer and a small Depth Transformer. Audio tokens and text tokens are processed in a two-stream design. The text tokens act as an Inner Monologue prefix, allowing the model to maintain semantic context while audio continues streaming."},
				{PaperID: "2410.00037v1", Version: "v1", Title: "Moshi: a speech-text foundation model for real-time dialogue", Section: "Codec", Page: "5", Text: "Mimi is a neural audio codec with a frame rate of 12.5 Hz, 8 quantizers, and a total bitrate of 1.1 kbps. This codec is designed for real-time speech and dialogue, compressing audio while preserving content needed for high-quality interaction."},
				{PaperID: "2410.00037v1", Version: "v1", Title: "Moshi: a speech-text foundation model for real-time dialogue", Section: "Latency", Page: "6", Text: "The system is designed for real-time, low-latency dialogue. Its theoretical latency is 160 ms, while practical full-duplex operation settles around 200 ms in end-to-end use. This is a substantial adaptation for streaming audio-language interaction."},
				{PaperID: "2410.00037v1", Version: "v1", Title: "Moshi: a speech-text foundation model for real-time dialogue", Section: "Narrative", Page: "2", Text: "Moshi is not a standard transcription model. Instead, it is a speech-text foundation model designed for real-time dialogue, seamlessly blending audio and text tokens in a streaming framework."},
			},
		},
		{
			ID:      "2507.07955v2",
			Version: "v2",
			Title:   "Dynamic Chunking for End-to-End Hierarchical Sequence Modeling",
			Passages: []Passage{
				{PaperID: "2507.07955v2", Version: "v2", Title: "Dynamic Chunking for End-to-End Hierarchical Sequence Modeling", Section: "Abstract", Page: "1", Text: "We propose H-Net, a hierarchical sequence model that replaces fixed tokenization with learned dynamic chunking. The system learns a routing module to decide chunk boundaries based on cosine similarity between adjacent hidden representations. A boundary probability p_t is produced during the forward pass and refined with a smoothing and interpolation module for differentiability. The model targets a compression ratio N and improves over fixed BPE tokenizers on byte-level sequence modeling."},
				{PaperID: "2507.07955v2", Version: "v2", Title: "Dynamic Chunking for End-to-End Hierarchical Sequence Modeling", Section: "Method", Page: "2", Text: "The routing module measures the cosine similarity between adjacent representations and produces a boundary probability p_t. To maintain differentiability, the model adds a smoothing and interpolation module that avoids a hard, non-differentiable decision boundary."},
				{PaperID: "2507.07955v2", Version: "v2", Title: "Dynamic Chunking for End-to-End Hierarchical Sequence Modeling", Section: "Compression", Page: "3", Text: "The framework targets a compression ratio N to balance information preservation and efficiency. This is arranged as a learned chunking objective rather than a fixed lexical tokenization scheme, allowing variable-length segments that adapt to local sequence structure."},
				{PaperID: "2507.07955v2", Version: "v2", Title: "Dynamic Chunking for End-to-End Hierarchical Sequence Modeling", Section: "Evaluation", Page: "4", Text: "The authors show that dynamic chunking outperforms fixed BPE tokenizers on byte-level sequence modeling tasks. This improvement is attributed to adaptive segmentation and hierarchical modeling rather than rigid predefined token boundaries."},
				{PaperID: "2507.07955v2", Version: "v2", Title: "Dynamic Chunking for End-to-End Hierarchical Sequence Modeling", Section: "Comparison", Page: "2", Text: "Compared with fixed tokenization, the dynamic chunking approach uses learned boundaries and routing decisions, replacing static byte-pair encoding assumptions with data-driven segmentation."},
			},
		},
	}

	out := make(map[string][]Passage, len(bundles))
	for _, bundle := range bundles {
		out[bundle.ID] = bundle.Passages
	}
	return out, nil
}

func PassageByID(id string) ([]Passage, error) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return nil, err
	}
	passages, ok := corpus[id]
	if !ok {
		return nil, fmt.Errorf("paper %s not found", id)
	}
	return passages, nil
}

func ChunkPassages(passages []Passage, pieceSize int) []Passage {
	if pieceSize <= 0 {
		pieceSize = 512
	}
	out := make([]Passage, 0, len(passages))
	for _, p := range passages {
		text := strings.TrimSpace(p.Text)
		if len(text) <= pieceSize {
			out = append(out, p)
			continue
		}
		for start := 0; start < len(text); start += pieceSize {
			end := start + pieceSize
			if end > len(text) {
				end = len(text)
			}
			chunk := p
			chunk.Text = text[start:end]
			out = append(out, chunk)
		}
	}
	return out
}
