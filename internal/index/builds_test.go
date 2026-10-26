package index

import (
	"os"
	"path/filepath"
	"testing"
)

// IDX-10: the runner's files are read; a missing file means no results.
func TestBuildsRead(t *testing.T) {
	dir := t.TempDir()
	bp, tp := filepath.Join(dir, "builds.json"), filepath.Join(dir, "tiers.json")
	os.WriteFile(bp, []byte(`[{"spec":"@a/b","version_sha":"sha1","tier":"haiku","model":"m","checks_passed":5,"checks_total":5,"cost_usd":0.02,"ran_at":"2026-10-26T10:00:00Z"}]`), 0o644)
	os.WriteFile(tp, []byte(`["haiku","sonnet"]`), 0o644)
	recs, err := ReadBuilds(bp)
	if err != nil || len(recs) != 1 || recs[0].CostUSD != 0.02 || recs[0].RanAt != "2026-10-26T10:00:00Z" {
		t.Fatalf("%+v %v", recs, err)
	}
	tiers, err := ReadTiers(tp)
	if err != nil || len(tiers) != 2 || tiers[0] != "haiku" {
		t.Fatalf("%v %v", tiers, err)
	}
	if recs, err := ReadBuilds(filepath.Join(dir, "none.json")); err != nil || len(recs) != 0 {
		t.Fatalf("missing file: %v %v", recs, err)
	}
	os.WriteFile(bp, []byte(`{bad`), 0o644)
	if _, err := ReadBuilds(bp); err == nil {
		t.Fatal("bad JSON accepted")
	}
}

func rec(tier string, passed, total int, cost float64) BuildRecord {
	return BuildRecord{Spec: "@a/b", VersionSHA: "sha1", Tier: tier, Model: tier + "-1", ChecksPassed: passed, ChecksTotal: total, CostUSD: cost}
}

func ixWith() *Index {
	return &Index{Specs: []Spec{{Name: "@a/b", Versions: []Version{{Version: "0.1.0", SHA: "sha1"}, {Version: "0.0.1", SHA: "sha0"}}}}}
}

// IDX-11
func TestBuildsWith(t *testing.T) {
	ix := ixWith()
	ApplyBuilds(ix, []string{"haiku", "sonnet"}, []BuildRecord{
		rec("sonnet", 5, 5, 0.50),
		rec("haiku", 4, 5, 0.01), // not all green
		rec("haiku", 5, 5, 0.09),
		rec("haiku", 5, 5, 0.04),
	})
	bw := ix.Specs[0].Versions[0].BuildsWith
	if bw == nil || bw.Tier != "haiku" || bw.CostUSD != 0.04 {
		t.Fatalf("builds_with %+v", bw)
	}
	if len(ix.Specs[0].Versions[0].Builds) != 4 {
		t.Fatalf("builds %+v", ix.Specs[0].Versions[0].Builds)
	}
	if ix.Specs[0].Versions[1].BuildsWith != nil {
		t.Fatal("unmeasured version got a result")
	}
}

// IDX-11: no all-green record, or zero checks → null.
func TestBuildsWithNone(t *testing.T) {
	ix := ixWith()
	ApplyBuilds(ix, []string{"haiku"}, []BuildRecord{rec("haiku", 3, 5, 0.01), rec("haiku", 0, 0, 0)})
	if ix.Specs[0].Versions[0].BuildsWith != nil {
		t.Fatalf("got %+v", ix.Specs[0].Versions[0].BuildsWith)
	}
	// Re-applying replaces what an earlier run attached.
	ApplyBuilds(ix, []string{"haiku"}, []BuildRecord{rec("haiku", 5, 5, 0.01)})
	if ix.Specs[0].Versions[0].BuildsWith == nil {
		t.Fatal("not re-derived")
	}
	ApplyBuilds(ix, []string{"haiku"}, nil)
	if ix.Specs[0].Versions[0].BuildsWith != nil || len(ix.Specs[0].Versions[0].Builds) != 0 {
		t.Fatal("stale result kept")
	}
}

// IDX-10
func TestBuildUnknownSha(t *testing.T) {
	ix := ixWith()
	r := rec("haiku", 5, 5, 0.01)
	r.VersionSHA = "nope"
	other := rec("haiku", 5, 5, 0.01)
	other.Spec = "@x/y"
	unknownTier := rec("tiny", 5, 5, 0.01)
	dropped := ApplyBuilds(ix, []string{"haiku"}, []BuildRecord{r, other, unknownTier})
	if dropped != 3 || ix.Specs[0].Versions[0].BuildsWith != nil || len(ix.Specs[0].Versions[0].Builds) != 0 {
		t.Fatalf("dropped %d, %+v", dropped, ix.Specs[0].Versions[0])
	}
}
