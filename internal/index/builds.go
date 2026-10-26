package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// BuildRecord is one build run by the runner, as stored in builds.json (IDX-10).
type BuildRecord struct {
	Spec         string  `json:"spec"`
	VersionSHA   string  `json:"version_sha"`
	Tier         string  `json:"tier"`
	Model        string  `json:"model"`
	ChecksPassed int     `json:"checks_passed"`
	ChecksTotal  int     `json:"checks_total"`
	CostUSD      float64 `json:"cost_usd"`
	RanAt        string  `json:"ran_at"`
}

// Build is a record as published on a version (IDX-12).
type Build struct {
	Tier         string  `json:"tier"`
	Model        string  `json:"model"`
	ChecksPassed int     `json:"checks_passed"`
	ChecksTotal  int     `json:"checks_total"`
	CostUSD      float64 `json:"cost_usd"`
}

// BuildsWith is the cheapest tier whose build got every check green (IDX-11).
type BuildsWith struct {
	Tier    string  `json:"tier"`
	CostUSD float64 `json:"cost_usd"`
}

// ReadBuilds reads builds.json; a missing file means no results yet.
func ReadBuilds(path string) ([]BuildRecord, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var recs []BuildRecord
	if err := json.Unmarshal(b, &recs); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return recs, nil
}

// ReadTiers reads tiers.json, a list of tier names, cheapest first; a missing file means no tiers.
func ReadTiers(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tiers []string
	if err := json.Unmarshal(b, &tiers); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return tiers, nil
}

// ApplyBuilds replaces the build results of every version from records (IDX-10, IDX-11).
// Records for a spec or version SHA the index doesn't have, or a tier not in tiers, are
// dropped; their count is returned. Calling it again starts from scratch, so results
// copied from the previous index never go stale.
func ApplyBuilds(ix *Index, tiers []string, recs []BuildRecord) (dropped int) {
	rank := map[string]int{}
	for i, t := range tiers {
		rank[t] = i
	}
	for i := range ix.Specs {
		for j := range ix.Specs[i].Versions {
			v := &ix.Specs[i].Versions[j]
			v.Builds, v.BuildsWith = []Build{}, nil
		}
	}
	for _, r := range recs {
		var ver *Version
		if sp, ok := ix.Find(r.Spec); ok {
			for j := range sp.Versions {
				if sp.Versions[j].SHA == r.VersionSHA {
					ver = &sp.Versions[j]
				}
			}
		}
		if _, known := rank[r.Tier]; ver == nil || !known {
			dropped++
			continue
		}
		ver.Builds = append(ver.Builds, Build{Tier: r.Tier, Model: r.Model, ChecksPassed: r.ChecksPassed, ChecksTotal: r.ChecksTotal, CostUSD: r.CostUSD})
		if r.ChecksTotal == 0 || r.ChecksPassed != r.ChecksTotal {
			continue
		}
		bw := ver.BuildsWith
		switch {
		case bw == nil, rank[r.Tier] < rank[bw.Tier]:
			ver.BuildsWith = &BuildsWith{Tier: r.Tier, CostUSD: r.CostUSD}
		case r.Tier == bw.Tier && r.CostUSD < bw.CostUSD:
			bw.CostUSD = r.CostUSD
		}
	}
	return dropped
}

// LatestBuildsWith is the result of the newest tagged version, or nil (IDX-12).
func (s Spec) LatestBuildsWith() *BuildsWith {
	if len(s.Versions) > 0 {
		return s.Versions[0].BuildsWith
	}
	return nil
}
