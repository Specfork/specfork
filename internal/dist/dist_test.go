// Package dist tests how the CLI is shipped: the npm launcher, install.sh and the Homebrew formula (CLI-12..15).
package dist

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const root = "../.."

func need(t *testing.T, tool string) {
	t.Helper()
	if _, err := exec.LookPath(tool); err != nil {
		t.Skipf("%s not installed", tool)
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell and node scripts are tested on unix")
	}
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func abs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(filepath.Join(root, p))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// hostPackage names the platform package for this host the way Node does.
func hostPackage() string {
	cpu := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	return "@specfork/cli-" + runtime.GOOS + "-" + cpu
}

// CLI-12
func TestNpmLauncher(t *testing.T) {
	need(t, "node")
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skip("no npm package for this architecture")
	}

	// A fake platform package next to the real launcher, laid out as npm installs them.
	nm := filepath.Join(t.TempDir(), "node_modules")
	for _, f := range []string{"bin/specfork.js", "lib.js", "package.json"} {
		b, err := os.ReadFile(abs(t, "npm/specfork/"+f))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(nm, "specfork", f), string(b), 0o755)
	}
	write(t, filepath.Join(nm, filepath.FromSlash(hostPackage()), "bin", "specfork"),
		"#!/bin/sh\necho \"args:$*\"\ncat\necho oops >&2\nexit 7\n", 0o755)

	cmd := exec.Command("node", filepath.Join(nm, "specfork", "bin", "specfork.js"), "resolve", "@specfork/todo", "--out", "x y")
	cmd.Stdin = strings.NewReader("from stdin\n")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 7 {
		t.Fatalf("want exit 7, got %v", err)
	}
	if out.String() != "args:resolve @specfork/todo --out x y\nfrom stdin\n" || errb.String() != "oops\n" {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}

	// Missing platform package: a clear message, exit 1.
	if err := os.RemoveAll(filepath.Join(nm, "@specfork")); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("node", filepath.Join(nm, "specfork", "bin", "specfork.js"), "--version")
	out.Reset()
	errb.Reset()
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 || !strings.Contains(errb.String(), "is not installed") {
		t.Errorf("missing package: %v, stderr %q", err, errb.String())
	}
}

// CLI-12, CLI-13: one package per CLI-11 target, one release version, no install scripts.
func TestNpmPackages(t *testing.T) {
	need(t, "node")
	bins, out := t.TempDir(), t.TempDir()
	var targets []struct{ OS, CPU, Goos, Goarch string }
	b, err := os.ReadFile(abs(t, "npm/targets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &targets); err != nil {
		t.Fatal(err)
	}
	if len(targets) != 6 {
		t.Fatalf("want 3 OS x 2 CPU, got %d targets", len(targets))
	}
	for _, tg := range targets {
		exe := "specfork"
		if tg.OS == "win32" {
			exe += ".exe"
		}
		write(t, filepath.Join(bins, tg.Goos+"_"+tg.Goarch, exe), "bin", 0o755)
	}
	if o, err := exec.Command("node", abs(t, "npm/build.js"), "1.2.3", bins, out).CombinedOutput(); err != nil {
		t.Fatalf("build.js: %v\n%s", err, o)
	}
	read := func(dir string) map[string]any {
		var m map[string]any
		b, err := os.ReadFile(filepath.Join(out, dir, "package.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	main := read("specfork")
	deps, _ := main["optionalDependencies"].(map[string]any)
	if main["name"] != "specfork" || main["version"] != "1.2.3" || len(deps) != 6 {
		t.Fatalf("main package %v", main)
	}
	if _, ok := main["scripts"]; ok {
		t.Error("main package has scripts")
	}
	for _, tg := range targets {
		name := "@specfork/cli-" + tg.OS + "-" + tg.CPU
		if deps[name] != "1.2.3" {
			t.Errorf("%s not pinned to the release version: %v", name, deps[name])
		}
		p := read("cli-" + tg.OS + "-" + tg.CPU)
		if p["name"] != name || p["version"] != "1.2.3" || fmt.Sprint(p["os"]) != "["+tg.OS+"]" || fmt.Sprint(p["cpu"]) != "["+tg.CPU+"]" {
			t.Errorf("%s: %v", name, p)
		}
		if _, ok := p["scripts"]; ok {
			t.Errorf("%s has scripts", name)
		}
	}
	for _, f := range []string{"specfork/bin/specfork.js", "specfork/lib.js", "cli-linux-x64/bin/specfork", "cli-win32-arm64/bin/specfork.exe"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
	if o, err := exec.Command("node", abs(t, "npm/build.js"), "latest", bins, out).CombinedOutput(); err == nil {
		t.Errorf("a non-version was accepted: %s", o)
	}
}

// release is a fake GitHub releases server for install.sh.
type release struct {
	assets map[string][]byte // path under /download/v1.2.3/ and /latest/download/
	hits   []string
}

func archive(t *testing.T, script string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "specfork", Mode: 0o755, Size: int64(len(script))}); err != nil {
		t.Fatal(err)
	}
	tw.Write([]byte(script))
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func newRelease(t *testing.T, tamper bool) (*release, *httptest.Server) {
	t.Helper()
	tgz := archive(t, "#!/bin/sh\necho specfork 1.2.3\n")
	listed := sum(tgz)
	if tamper {
		listed = sum([]byte("something else"))
	}
	r := &release{assets: map[string][]byte{
		"specfork_linux_amd64.tar.gz": tgz,
		"checksums.txt":               []byte(listed + "  specfork_linux_amd64.tar.gz\n" + sum(nil) + "  specfork_darwin_arm64.tar.gz\n"),
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.hits = append(r.hits, req.URL.Path)
		for _, prefix := range []string{"/latest/download/", "/download/v1.2.3/"} {
			if name, ok := strings.CutPrefix(req.URL.Path, prefix); ok {
				if b, ok := r.assets[name]; ok {
					w.Write(b)
					return
				}
			}
		}
		http.NotFound(w, req)
	}))
	t.Cleanup(srv.Close)
	return r, srv
}

// runInstall runs install.sh with a fake uname (the host is not the target) and returns its output and exit code.
// An env entry "PATH+=dir" adds dir to PATH.
func runInstall(t *testing.T, uname [2]string, env ...string) (string, int) {
	t.Helper()
	bin := t.TempDir()
	write(t, filepath.Join(bin, "uname"),
		"#!/bin/sh\ncase \"$1\" in -s) echo "+uname[0]+";; -m) echo "+uname[1]+";; esac\n", 0o755)
	path := bin + ":" + os.Getenv("PATH")
	cmd := exec.Command("sh", abs(t, "install.sh"))
	cmd.Env = []string{"HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir()}
	for _, e := range env {
		if dir, ok := strings.CutPrefix(e, "PATH+="); ok { // an extra PATH entry, behind the fake uname
			path = bin + ":" + dir + ":" + os.Getenv("PATH")
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	cmd.Env = append(cmd.Env, "PATH="+path)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

// CLI-15
func TestInstallScript(t *testing.T) {
	need(t, "sh")
	linux := [2]string{"Linux", "x86_64"}

	t.Run("ok", func(t *testing.T) {
		r, srv := newRelease(t, false)
		dir := filepath.Join(t.TempDir(), "bin")
		out, code := runInstall(t, linux, "SPECFORK_RELEASES="+srv.URL, "SPECFORK_INSTALL="+dir)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		got, err := exec.Command(filepath.Join(dir, "specfork")).Output()
		if err != nil || string(got) != "specfork 1.2.3\n" {
			t.Fatalf("installed binary: %q, %v", got, err)
		}
		// The directory is not on PATH, so the script says how to add it.
		if !strings.Contains(out, "export PATH=\""+dir+":") {
			t.Errorf("no PATH hint: %s", out)
		}
		if strings.Join(r.hits, " ") != "/latest/download/specfork_linux_amd64.tar.gz /latest/download/checksums.txt" {
			t.Errorf("requests %v", r.hits)
		}
	})

	t.Run("default dir and pinned version", func(t *testing.T) {
		r, srv := newRelease(t, false)
		home := t.TempDir()
		bin := t.TempDir()
		write(t, filepath.Join(bin, "uname"), "#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo aarch64;; esac\n", 0o755)
		// aarch64 maps to arm64, which the fake server does not have: the download must fail cleanly.
		cmd := exec.Command("sh", abs(t, "install.sh"))
		cmd.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + t.TempDir(), "SPECFORK_RELEASES=" + srv.URL, "SPECFORK_VERSION=v1.2.3"}
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "could not download") {
			t.Fatalf("want a download error, got %v: %s", err, out)
		}
		if len(r.hits) == 0 || r.hits[0] != "/download/v1.2.3/specfork_linux_arm64.tar.gz" {
			t.Errorf("requests %v", r.hits)
		}
		if _, err := os.Stat(filepath.Join(home, ".local", "bin", "specfork")); err == nil {
			t.Error("installed after a failed download")
		}
		// Same server, amd64: lands in ~/.local/bin.
		write(t, filepath.Join(bin, "uname"), "#!/bin/sh\ncase \"$1\" in -s) echo Linux;; -m) echo x86_64;; esac\n", 0o755)
		cmd = exec.Command("sh", abs(t, "install.sh"))
		cmd.Env = []string{"PATH=" + bin + ":" + os.Getenv("PATH"), "HOME=" + home, "TMPDIR=" + t.TempDir(), "SPECFORK_RELEASES=" + srv.URL, "SPECFORK_VERSION=1.2.3"}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(home, ".local", "bin", "specfork")); err != nil {
			t.Error("not installed in ~/.local/bin")
		}
	})

	t.Run("bad checksum", func(t *testing.T) {
		_, srv := newRelease(t, true)
		dir := filepath.Join(t.TempDir(), "bin")
		out, code := runInstall(t, linux, "SPECFORK_RELEASES="+srv.URL, "SPECFORK_INSTALL="+dir)
		if code == 0 || !strings.Contains(out, "checksum mismatch") {
			t.Fatalf("exit %d: %s", code, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "specfork")); err == nil {
			t.Error("binary installed despite a bad checksum")
		}
	})

	t.Run("unsupported OS", func(t *testing.T) {
		r, srv := newRelease(t, false)
		out, code := runInstall(t, [2]string{"FreeBSD", "amd64"}, "SPECFORK_RELEASES="+srv.URL, "SPECFORK_INSTALL="+t.TempDir())
		if code == 0 || !strings.Contains(out, "unsupported OS") || len(r.hits) != 0 {
			t.Fatalf("exit %d, requests %v: %s", code, r.hits, out)
		}
		out, code = runInstall(t, [2]string{"Linux", "riscv64"}, "SPECFORK_RELEASES="+srv.URL, "SPECFORK_INSTALL="+t.TempDir())
		if code == 0 || !strings.Contains(out, "unsupported architecture") {
			t.Fatalf("exit %d: %s", code, out)
		}
	})

	t.Run("custom dir on PATH", func(t *testing.T) {
		_, srv := newRelease(t, false)
		dir := t.TempDir()
		out, code := runInstall(t, linux, "SPECFORK_RELEASES="+srv.URL, "SPECFORK_INSTALL="+dir, "PATH+="+dir)
		if code != 0 {
			t.Fatalf("exit %d: %s", code, out)
		}
		if strings.Contains(out, "not on your PATH") {
			t.Errorf("PATH hint although the directory is on PATH: %s", out)
		}
	})

	t.Run("never sudo", func(t *testing.T) {
		b, err := os.ReadFile(abs(t, "install.sh"))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if l = strings.TrimSpace(l); !strings.HasPrefix(l, "#") && strings.Contains(l, "sudo") {
				t.Errorf("install.sh uses sudo: %s", l)
			}
		}
	})
}

// CLI-14
func TestBrewFormula(t *testing.T) {
	need(t, "sh")
	sums := filepath.Join(t.TempDir(), "checksums.txt")
	var lines []string
	for _, p := range []string{"darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64", "windows_amd64"} {
		lines = append(lines, sum([]byte(p))+"  specfork_"+p+".tar.gz")
	}
	write(t, sums, strings.Join(lines, "\n")+"\n", 0o644)
	out, err := exec.Command("sh", abs(t, "scripts/brew-formula.sh"), "v1.2.3", sums).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	f := string(out)
	for _, want := range []string{
		`class Specfork < Formula`, `version "1.2.3"`,
		`releases/download/v1.2.3/specfork_darwin_arm64.tar.gz"`, `sha256 "` + sum([]byte("darwin_arm64")) + `"`,
		`releases/download/v1.2.3/specfork_linux_amd64.tar.gz"`, `sha256 "` + sum([]byte("linux_amd64")) + `"`,
		`bin.install "specfork"`,
	} {
		if !strings.Contains(f, want) {
			t.Errorf("formula lacks %q:\n%s", want, f)
		}
	}
	if o, err := exec.Command("sh", abs(t, "scripts/brew-formula.sh"), "1.2.3", filepath.Join(t.TempDir(), "none")).CombinedOutput(); err == nil {
		t.Errorf("missing checksums accepted: %s", o)
	}
}

// CLI-10, CLI-11, CLI-13, CLI-14
func TestReleaseWorkflow(t *testing.T) {
	b, err := os.ReadFile(abs(t, ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	wf := string(b)
	for _, want := range []string{
		"googleapis/release-please-action", "scripts/build-release.sh", "actions/attest-build-provenance",
		"id-token: write", "npm publish", "--provenance", "scripts/brew-formula.sh", "Specfork/homebrew-tap",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("release.yml lacks %q", want)
		}
	}
	for _, bad := range []string{"NPM_TOKEN", "NODE_AUTH_TOKEN"} {
		if strings.Contains(wf, bad) {
			t.Errorf("release.yml uses a stored npm token (%s)", bad)
		}
	}
	if platform, main := strings.Index(wf, "npm publish \"$d\""), strings.Index(wf, "npm publish dist/npm/specfork"); platform < 0 || main < platform {
		t.Error("the specfork package must be published after the platform packages")
	}
	var cfg struct {
		Packages map[string]struct {
			ReleaseType string `json:"release-type"`
			Component   bool   `json:"include-component-in-tag"`
		} `json:"packages"`
	}
	c, err := os.ReadFile(abs(t, "release-please-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(c, &cfg); err != nil || cfg.Packages["."].ReleaseType == "" || cfg.Packages["."].Component {
		t.Errorf("release-please config: %v %+v", err, cfg)
	}
}
