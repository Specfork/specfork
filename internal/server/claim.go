package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/specfork/specfork/internal/ghapi"
	"github.com/specfork/specfork/internal/registry"
	"github.com/specfork/specfork/internal/spec"
)

const maxClaimRepos = 20

// candidate is the names.json entry a repo would claim, from its README front matter name.
func (s *Server) candidate(gh *ghapi.Client, repo ghapi.Repo) (string, registry.Entry, error) {
	e := registry.Entry{RepoID: repo.ID, OwnerID: repo.Owner.ID, Repo: repo.FullName, ClaimedAt: s.now().UTC().Format("2006-01-02"), State: "active"}
	sha, err := gh.BranchSHA(repo.FullName, repo.DefaultBranch)
	if err != nil {
		return "", e, err
	}
	readme, err := gh.Raw(repo.FullName, sha, "README.md")
	if err != nil {
		return "", e, err
	}
	fm, err := spec.ParseFrontMatter(readme)
	if err != nil {
		return "", e, fmt.Errorf("README.md: %w", err)
	}
	return fm.Name, e, nil
}

// names reads the registry's current names.json (REG-11: the file is the source of truth).
func (s *Server) names(gh *ghapi.Client) (*registry.Names, []byte, error) {
	raw, err := gh.Raw(s.cfg.RegistryRepo, s.cfg.RegistryRef, "registry/names.json")
	if err != nil {
		return nil, nil, err
	}
	n, err := registry.Parse(raw)
	return n, raw, err
}

// check runs the REG-2 checks for one repo with the signed-in user as claimant (API-23, REG-12).
func (s *Server) check(gh *ghapi.Client, base *registry.Names, repo ghapi.Repo, claimant string) (string, registry.Entry, []string) {
	name, e, err := s.candidate(gh, repo)
	if err != nil {
		return name, e, []string{"can't read the README front matter of " + repo.FullName + ": " + err.Error()}
	}
	head := &registry.Names{Names: map[string]registry.Entry{name: e}}
	for k, v := range base.Names {
		head.Names[k] = v
	}
	if _, taken := base.Names[name]; taken {
		return name, e, []string{name + ": the name is already claimed"}
	}
	v := registry.CheckClaim(base, head, claimant, gh)
	return name, e, v.Problems
}

type claimable struct {
	Repo      string   `json:"repo"`
	RepoID    int64    `json:"repo_id"`
	Name      string   `json:"name"`
	Claimable bool     `json:"claimable"`
	Failures  []string `json:"failures"`
}

// claimRepos lists the user's own repos with the specfork topic that are not claimed yet (API-22).
func (s *Server) claimRepos(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.user(w, r)
	if !ok {
		return
	}
	gh, err := s.app()
	if err != nil {
		githubDown(w)
		return
	}
	u, err := gh.UserByID(id)
	if err != nil {
		githubDown(w)
		return
	}
	base, _, err := s.names(gh)
	if err != nil {
		githubDown(w)
		return
	}
	repos, err := gh.SearchRepos("topic:specfork fork:true user:" + u.Login)
	if err != nil {
		githubDown(w)
		return
	}
	out := []claimable{}
	for _, repo := range repos {
		if _, _, taken := base.ByRepoID(repo.ID); taken || len(out) >= maxClaimRepos || !strings.EqualFold(repo.Owner.Login, u.Login) {
			continue
		}
		name, _, problems := s.check(gh, base, repo, u.Login)
		out = append(out, claimable{Repo: repo.FullName, RepoID: repo.ID, Name: name, Claimable: len(problems) == 0, Failures: append([]string{}, problems...)})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// branchFor is the branch of a claim PR; "@" and "/" are valid in git ref names.
func branchFor(name string) string { return "claim/" + name }

// claim runs the checks and opens the PR to names.json as the GitHub App (API-23..26).
func (s *Server) claim(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.user(w, r)
	if !ok || !s.throttle(w, id) {
		return
	}
	var body struct {
		RepoID int64 `json:"repo_id"`
	}
	b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<10))
	if json.Unmarshal(b, &body) != nil || body.RepoID <= 0 {
		writeErr(w, http.StatusBadRequest, "repo_id_required")
		return
	}
	gh, err := s.app()
	if err != nil {
		githubDown(w)
		return
	}
	u, err := gh.UserByID(id)
	if err != nil {
		githubDown(w)
		return
	}
	repo, err := gh.RepoByID(body.RepoID)
	if errors.Is(err, ghapi.ErrNotFound) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "claim_failed", "failures": []string{"repo not found or not visible to Specfork"}})
		return
	} else if err != nil {
		githubDown(w)
		return
	}
	base, raw, err := s.names(gh)
	if err != nil {
		githubDown(w)
		return
	}
	name, entry, problems := s.check(gh, base, repo, u.Login)
	if len(problems) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "claim_failed", "failures": problems})
		return
	}
	owner, reg, _ := strings.Cut(s.cfg.RegistryRepo, "/")
	branch := branchFor(name)
	if url, found, err := gh.FindOpenPR(owner, reg, branch); err != nil {
		githubDown(w)
		return
	} else if found {
		writeJSON(w, http.StatusOK, map[string]string{"pr_url": url})
		return
	}
	next, err := registry.AddEntry(raw, name, entry)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "names_json")
		return
	}
	url, err := s.openPR(gh, branch, name, id, next)
	if err != nil {
		githubDown(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"pr_url": url})
}

// openPR pushes a branch with the one new entry and opens the PR (API-24).
func (s *Server) openPR(gh *ghapi.Client, branch, name string, claimant int64, names []byte) (string, error) {
	full, ref := s.cfg.RegistryRepo, s.cfg.RegistryRef
	head, err := gh.Ref(full, ref)
	if err != nil {
		return "", err
	}
	blob, err := gh.FileSHA(full, "registry/names.json", ref)
	if err != nil {
		return "", err
	}
	if err := gh.SetBranch(full, branch, head); err != nil {
		return "", err
	}
	title := "chore(registry): claim " + name
	if err := gh.PutFile(full, "registry/names.json", branch, title, names, blob); err != nil {
		return "", err
	}
	body := fmt.Sprintf("Claim %s, opened from specfork.dev.\n\nClaimant: %d\n", name, claimant)
	return gh.OpenPR(full, branch, ref, title, body)
}
