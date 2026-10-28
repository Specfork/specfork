// Package scores holds the public score document served at /scores.json (API-20) and read by the site build.
package scores

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// Score is one spec's average and number of scores.
type Score struct {
	Spec    string  `json:"spec"`
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

// Doc is /scores.json.
type Doc struct {
	GeneratedAt string  `json:"generated_at"`
	Scores      []Score `json:"scores"`
}

// Average is sum/count rounded to 2 decimals.
func Average(sum, count int64) float64 {
	return math.Round(float64(sum)/float64(count)*100) / 100
}

// Fetch reads a /scores.json document; the crawl treats a failure as "no scores" (IDX-15).
func Fetch(url string) ([]Score, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	res, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var d Doc
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return d.Scores, nil
}

// BySpec indexes scores by spec name, dropping empty ones: no data is never shown as 0 (WEB-18).
func BySpec(list []Score) map[string]Score {
	out := map[string]Score{}
	for _, s := range list {
		if s.Count > 0 && s.Average >= 1 {
			out[s.Spec] = s
		}
	}
	return out
}
