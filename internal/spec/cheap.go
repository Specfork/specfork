package spec

import (
	"fmt"
	"regexp"
	"strings"
)

// Limits of the cheap-buildable profile (FMT-13).
const (
	CheapMaxRows  = 15
	CheapMaxLines = 60
)

var (
	sepRowRe = regexp.MustCompile(`^\|[\s:|-]+\|$`)
	vagueRe  = regexp.MustCompile(`(?i)\b(usually|typically|reasonable|appropriate|sensible|suitable|as needed|if needed|where possible|and so on|etc|tbd|somehow|various)\b`)
	wordRes  = map[string]*regexp.Regexp{
		"given": regexp.MustCompile(`(?i)\bgiven\b`),
		"when":  regexp.MustCompile(`(?i)\bwhen\b`),
		"then":  regexp.MustCompile(`(?i)\bthen\b`),
	}
)

// LintCheap checks FMT-10..14, the opt-in cheap-buildable profile. The checks are
// mechanical (sections, shapes, size, vague words); whether a cheap model really
// builds the spec shows only in build results (IDX-10).
func LintCheap(r Repo) []Issue {
	var out []Issue
	for _, a := range r.Areas {
		add := func(line int, rule, msg string) {
			out = append(out, Issue{File: a.File, Line: line, Rule: rule, Message: msg})
		}
		lines := strings.Split(strings.TrimRight(a.Raw, "\n"), "\n")

		// FMT-10
		body, at, ok := sectionOf(lines, "Data model")
		switch {
		case !ok:
			add(0, "FMT-10", "missing `## Data model` (list every entity and field with type, required, limits; write `None.` if the area stores nothing)")
		case contentLines(body) == 0:
			add(at, "FMT-10", "`## Data model` is empty")
		}

		// FMT-11
		body, at, ok = sectionOf(lines, "Examples")
		if !ok {
			add(0, "FMT-11", "missing `## Examples` (concrete input → exact expected output)")
		} else if !hasArrow(body) {
			add(at, "FMT-11", "`## Examples` has no worked example (a line with `→`: input values → expected output)")
		}

		live := 0
		for _, row := range a.Rows {
			if row.Removed() {
				continue
			}
			live++
			// FMT-12
			if !executable(row.VerifiedBy) {
				add(row.Line, "FMT-12", row.ID+": Verified by must be a `given … when … then …` scenario or a command with its expected result (→ exit code or output), not `manual` or a bare test name")
			}
			// FMT-14
			if m := vagueRe.FindAllString(row.Requirement, -1); len(m) > 0 {
				seen := map[string]bool{}
				var words []string
				for _, w := range m {
					if k := strings.ToLower(w); !seen[k] {
						seen[k] = true
						words = append(words, fmt.Sprintf("%q", k))
					}
				}
				add(row.Line, "FMT-14", fmt.Sprintf("%s leaves a choice to the builder (%s): state the value, default or behaviour", row.ID, strings.Join(words, ", ")))
			}
		}

		// FMT-13
		if live > CheapMaxRows {
			add(0, "FMT-13", fmt.Sprintf("%d rows, max %d: split the area", live, CheapMaxRows))
		}
		if len(lines) > CheapMaxLines {
			add(0, "FMT-13", fmt.Sprintf("%d lines, max %d: split the area", len(lines), CheapMaxLines))
		}
	}
	return out
}

// sectionOf returns the lines under "## title" up to the next heading, and the heading's line number.
func sectionOf(lines []string, title string) (body []string, at int, ok bool) {
	for i, l := range lines {
		if ok {
			if strings.HasPrefix(l, "## ") || strings.HasPrefix(l, "# ") {
				break
			}
			body = append(body, l)
		} else if strings.EqualFold(strings.TrimSpace(l), "## "+title) {
			ok, at = true, i+1
		}
	}
	return body, at, ok
}

// contentLines counts lines that carry content: a table's header and separator rows don't.
func contentLines(body []string) int {
	n, header := 0, false
	for _, l := range body {
		l = strings.TrimSpace(l)
		switch {
		case l == "", sepRowRe.MatchString(l):
		case strings.HasPrefix(l, "|") && !header:
			header = true
		default:
			n++
		}
	}
	return n
}

func hasArrow(body []string) bool {
	for _, l := range body {
		if strings.Contains(l, "→") || strings.Contains(l, "->") {
			return true
		}
	}
	return false
}

// executable reports whether a Verified by cell is a scenario or a command with a result (FMT-12).
func executable(v string) bool {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(v), "manual") {
		return false
	}
	if wordRes["given"].MatchString(v) && wordRes["when"].MatchString(v) && wordRes["then"].MatchString(v) {
		return true
	}
	return strings.Contains(v, "`") && (strings.Contains(v, "→") || strings.Contains(v, "->") || strings.Contains(strings.ToLower(v), "exit"))
}
