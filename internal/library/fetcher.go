package library

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

func FetchAndPrepareCorpus() error {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return err
	}
	if len(corpus) == 0 {
		return fmt.Errorf("library corpus is empty")
	}
	return nil
}

func DownloadArxivHTML(id string) (string, error) {
	url := fmt.Sprintf("https://export.arxiv.org/api/query?id_list=%s", id)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("arxiv fetch failed: status %d", resp.StatusCode)
	}
	var b bytes.Buffer
	if _, err := io.Copy(&b, resp.Body); err != nil {
		return "", err
	}
	return b.String(), nil
}

func BestEffortPrepare() []Passage {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		return nil
	}
	out := make([]Passage, 0, len(corpus))
	for _, p := range corpus {
		out = append(out, p...)
	}
	return out
}
