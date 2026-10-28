package registry

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// DefaultBot is the login of the Specfork GitHub App on pull requests (REG-9).
const DefaultBot = "specfork[bot]"

var claimantRe = regexp.MustCompile(`(?m)^Claimant: ([0-9]+)[ \t]*\r?$`)

// ClaimantID reads the one `Claimant: <github_id>` line of a PR body.
func ClaimantID(body string) (int64, bool) {
	m := claimantRe.FindAllStringSubmatch(body, -1)
	if len(m) != 1 {
		return 0, false
	}
	id, err := strconv.ParseInt(m[0][1], 10, 64)
	return id, err == nil && id > 0
}

// Users resolves a GitHub account ID to its current login.
type Users interface {
	LoginByID(id int64) (string, error)
}

// Author is whose rights a claim PR is checked against: the PR author, or for a PR opened by
// the Specfork App the claimant named in its body (REG-9). The line is ignored on other PRs.
func Author(author, body, bot string, users Users) (string, []string) {
	if author != bot {
		return author, nil
	}
	id, ok := ClaimantID(body)
	if !ok {
		return "", []string{"a PR from " + bot + " needs exactly one `Claimant: <github_id>` line"}
	}
	login, err := users.LoginByID(id)
	if err != nil {
		return "", []string{fmt.Sprintf("can't resolve claimant %d: %v", id, err)}
	}
	return login, nil
}

// FileProblems refuses a claim PR that changes anything but names.json (REG-2, REG-10).
func FileProblems(files []string) []string {
	var out []string
	for _, f := range files {
		if f != "registry/names.json" {
			out = append(out, "a claim PR may only change registry/names.json, not "+f)
		}
	}
	return out
}

// AddEntry returns names.json with one more entry; every other entry stays as it was (REG-11).
func AddEntry(raw []byte, name string, e Entry) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("names.json: %w", err)
	}
	var names map[string]json.RawMessage
	if err := json.Unmarshal(doc["names"], &names); err != nil {
		return nil, fmt.Errorf("names.json: %w", err)
	}
	if _, taken := names[name]; taken {
		return nil, fmt.Errorf("%s is already in names.json", name)
	}
	b, err := json.Marshal(map[string]any{"repo_id": e.RepoID, "owner_id": e.OwnerID, "repo": e.Repo, "claimed_at": e.ClaimedAt, "state": e.State})
	if err != nil {
		return nil, err
	}
	names[name] = b
	if doc["names"], err = json.Marshal(names); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	return append(out, '\n'), err
}
