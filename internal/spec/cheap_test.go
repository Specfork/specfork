package spec

import (
	"strings"
	"testing"
)

const goodArea = `# Pets

**Status:** Draft · **Prefix:** PET

## Purpose

Keep a list of pets.

## Data model

| Entity | Field | Type | Required | Limits |
|---|---|---|---|---|
| Pet | name | text | yes | 1-40 chars |

## Requirements

| ID | Requirement | Verified by |
|---|---|---|
| PET-1 | Adding a pet stores it; a name over 40 chars is rejected with error ` + "`name_too_long`" + `. | given no pets, when I add "Rex", then the list is ["Rex"] |
| PET-2 | Removed: superseded. | manual |

## Examples

- add "Rex" → list ["Rex"]
`

func cheapRules(area string) string {
	issues := LintCheap(Repo{Areas: []Area{ParseArea("specs/pets.md", []byte(area))}})
	var out []string
	for _, i := range issues {
		out = append(out, i.String())
	}
	return strings.Join(out, "\n")
}

func TestLintCheapClean(t *testing.T) {
	if got := cheapRules(goodArea); got != "" {
		t.Fatalf("clean area flagged:\n%s", got)
	}
}

// FMT-10
func TestLintCheapDataModel(t *testing.T) {
	bad := strings.Replace(goodArea, "| Pet | name | text | yes | 1-40 chars |", "", 1)
	if got := cheapRules(bad); !strings.Contains(got, "FMT-10") {
		t.Errorf("empty data model not flagged:\n%s", got)
	}
	bad = strings.Replace(goodArea, "## Data model", "## Model", 1)
	if got := cheapRules(bad); !strings.Contains(got, "FMT-10") {
		t.Errorf("missing data model not flagged:\n%s", got)
	}
}

// FMT-11
func TestLintCheapExamples(t *testing.T) {
	bad := strings.Replace(goodArea, "- add \"Rex\" → list [\"Rex\"]", "- some text", 1)
	if got := cheapRules(bad); !strings.Contains(got, "FMT-11") {
		t.Errorf("example without input → output not flagged:\n%s", got)
	}
	bad = strings.Replace(goodArea, "## Examples", "## Notes", 1)
	if got := cheapRules(bad); !strings.Contains(got, "FMT-11") {
		t.Errorf("missing examples not flagged:\n%s", got)
	}
}

// FMT-12
func TestLintCheapExecutable(t *testing.T) {
	bad := strings.Replace(goodArea, "given no pets, when I add \"Rex\", then the list is [\"Rex\"]", "manual: look at the list", 1)
	if got := cheapRules(bad); !strings.Contains(got, "PET-1") || !strings.Contains(got, "FMT-12") {
		t.Errorf("manual check not flagged:\n%s", got)
	}
	bad = strings.Replace(goodArea, "given no pets, when I add \"Rex\", then the list is [\"Rex\"]", "TestAddPet", 1)
	if got := cheapRules(bad); !strings.Contains(got, "FMT-12") {
		t.Errorf("bare test name is not a scenario:\n%s", got)
	}
	ok := strings.Replace(goodArea, "given no pets, when I add \"Rex\", then the list is [\"Rex\"]", "`pets add Rex` → exit 0, stdout `Rex`", 1)
	if got := cheapRules(ok); got != "" {
		t.Errorf("command with expected output flagged:\n%s", got)
	}
}

// FMT-13
func TestLintCheapSize(t *testing.T) {
	var rows strings.Builder
	for i := 2; i <= 17; i++ {
		rows.WriteString("| PET-" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + " | Pets stay. | given a pet, when I wait, then it stays |\n")
	}
	bad := strings.Replace(goodArea, "| PET-2 | Removed: superseded. | manual |\n", rows.String(), 1)
	if got := cheapRules(bad); !strings.Contains(got, "FMT-13") || !strings.Contains(got, "rows") {
		t.Errorf("17 rows not flagged:\n%s", got)
	}
	long := goodArea + strings.Repeat("filler\n", 60)
	if got := cheapRules(long); !strings.Contains(got, "FMT-13") || !strings.Contains(got, "lines") {
		t.Errorf("long area not flagged:\n%s", got)
	}
}

// FMT-14
func TestLintCheapDefaults(t *testing.T) {
	bad := strings.Replace(goodArea, "Adding a pet stores it", "Adding a pet stores it, usually with a reasonable default", 1)
	got := cheapRules(bad)
	if !strings.Contains(got, "FMT-14") || !strings.Contains(got, "usually") || !strings.Contains(got, "reasonable") {
		t.Errorf("vague wording not flagged:\n%s", got)
	}
}
