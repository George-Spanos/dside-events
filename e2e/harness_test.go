package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// server is one running `dside-events serve` process with its own SQLite file.
type server struct {
	dir    string
	dbPath string
	url    string // http://127.0.0.1:PORT, no trailing slash

	cmd  *exec.Cmd
	done chan struct{}

	mu     sync.Mutex
	log    bytes.Buffer
	dumped int
}

var listeningLine = regexp.MustCompile(`^listening on (http://127\.0\.0\.1:\d+)\s*$`)

// startServer launches a fresh server (empty database) for one test and stops
// it on cleanup. Its log is attached to the test output when the test fails.
func startServer(t *testing.T) *server {
	t.Helper()
	dir, err := os.MkdirTemp("", "dside-e2e-test-")
	if err != nil {
		t.Fatal(err)
	}
	s, err := launchServer(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		s.stop()
		if t.Failed() {
			t.Logf("server log:\n%s", s.logs())
		}
		os.RemoveAll(dir)
	})
	return s
}

// launchServer starts `serve` in dir with `ADDR=127.0.0.1:0 DB_PATH=…` and
// waits for the listening line and a healthy /healthz.
func launchServer(dir string) (*server, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &server{
		dir:    dir,
		dbPath: filepath.Join(dir, "events.db"),
		done:   make(chan struct{}),
	}
	cmd := exec.Command(binPath, "serve")
	cmd.Env = append(os.Environ(),
		"ADDR=127.0.0.1:0",
		"DB_PATH="+s.dbPath,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s serve: %w", binPath, err)
	}
	s.cmd = cmd

	ready := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		found := false
		for sc.Scan() {
			line := sc.Text()
			s.append("stdout: " + line + "\n")
			if !found {
				if m := listeningLine.FindStringSubmatch(line); m != nil {
					found = true
					ready <- m[1]
				}
			}
		}
	}()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				s.append(string(buf[:n]))
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		cmd.Wait()
		close(s.done)
	}()

	select {
	case u := <-ready:
		s.url = u
	case <-s.done:
		return nil, fmt.Errorf("server exited before listening:\n%s", s.logs())
	case <-time.After(20 * time.Second):
		s.stop()
		return nil, fmt.Errorf("timed out waiting for 'listening on' line:\n%s", s.logs())
	}
	if err := waitHealthy(s.url); err != nil {
		s.stop()
		return nil, fmt.Errorf("%v\n%s", err, s.logs())
	}
	return s, nil
}

func waitHealthy(base string) error {
	deadline := time.Now().Add(10 * time.Second)
	hc := &http.Client{Timeout: 2 * time.Second}
	var last string
	for time.Now().Before(deadline) {
		resp, err := hc.Get(base + "/healthz")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Sprintf("%d %s", resp.StatusCode, strings.TrimSpace(string(b)))
		} else {
			last = err.Error()
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("/healthz never returned 200 (last: %s)", last)
}

// stop sends SIGTERM and kills after five seconds.
func (s *server) stop() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-s.done:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.done
	}
}

func (s *server) append(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.WriteString(text)
}

func (s *server) logs() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.log.String()
}

// dumpNewLogs attaches the log lines produced since the previous dump to t.
// Used by the shared server so a failing test shows only its own traffic.
func (s *server) dumpNewLogs(t testing.TB) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dumped > s.log.Len() {
		s.dumped = 0
	}
	fresh := s.log.String()[s.dumped:]
	s.dumped = s.log.Len()
	if strings.TrimSpace(fresh) != "" {
		t.Logf("server log since previous test:\n%s", fresh)
	}
}

// ---- CLI --------------------------------------------------------------------

const (
	// linkUnchanged is the second line `add-poster` prints for a slug that
	// already exists.
	linkUnchanged = "link (unchanged, run poster-link to get a new one)"
)

var (
	keyPattern = `[A-Za-z0-9_-]{43}`
	posterLine = regexp.MustCompile(`(?m)^poster (\S+)\s*$`)
	linkLine   = regexp.MustCompile(`(?m)^link (\S+)\s*$`)
	linkKey    = regexp.MustCompile(`/k/(` + keyPattern + `)$`)
)

// runCLI runs the binary with args against s's database, with BASE_URL set to
// s's address so printed links open on that server. It returns stdout.
func runCLI(s *server, args ...string) (string, error) {
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), "DB_PATH="+s.dbPath, "BASE_URL="+s.url)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %v\nstdout: %s\nstderr: %s", strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), nil
}

// parseLink extracts `link <url>` from CLI output and the key inside the url.
func parseLink(out string) (link, key string, err error) {
	m := linkLine.FindStringSubmatch(out)
	if m == nil {
		return "", "", fmt.Errorf("stdout lacks 'link <url>':\n%s", out)
	}
	k := linkKey.FindStringSubmatch(m[1])
	if k == nil {
		return "", "", fmt.Errorf("link %q does not end in /k/<43-char key>", m[1])
	}
	return m[1], k[1], nil
}

// runAddPoster runs `dside-events add-poster -name <name> [-slug <slug>]`
// against s and parses the two output lines `poster <slug>` and `link <url>`.
// It fails for a no-op run (existing slug); use addPosterRaw for that case.
func runAddPoster(s *server, name, slug string) (poster, error) {
	args := []string{"add-poster", "-name", name}
	if slug != "" {
		args = append(args, "-slug", slug)
	}
	out, err := runCLI(s, args...)
	if err != nil {
		return poster{}, fmt.Errorf("add-poster %q: %w", name, err)
	}
	m := posterLine.FindStringSubmatch(out)
	if m == nil {
		return poster{}, fmt.Errorf("add-poster %q: stdout lacks 'poster <slug>':\n%s", name, out)
	}
	link, key, err := parseLink(out)
	if err != nil {
		return poster{}, fmt.Errorf("add-poster %q: %v", name, err)
	}
	return poster{Name: name, Slug: m[1], Link: link, Key: key}, nil
}

// addPoster is runAddPoster for use inside a test.
func addPoster(t testing.TB, s *server, name, slug string) poster {
	t.Helper()
	p, err := runAddPoster(s, name, slug)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// addPosterRaw runs add-poster and returns its raw stdout (for the no-op case).
func addPosterRaw(t testing.TB, s *server, name, slug string) string {
	t.Helper()
	args := []string{"add-poster", "-name", name}
	if slug != "" {
		args = append(args, "-slug", slug)
	}
	out, err := runCLI(s, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// posterLink runs `dside-events poster-link -slug <slug>` against s and
// returns the new link and its key.
func posterLink(t testing.TB, s *server, slug string) (link, key string) {
	t.Helper()
	out, err := runCLI(s, "poster-link", "-slug", slug)
	if err != nil {
		t.Fatal(err)
	}
	link, key, err = parseLink(out)
	if err != nil {
		t.Fatalf("poster-link -slug %s: %v", slug, err)
	}
	if strings.Contains(out, "poster ") && posterLine.MatchString(out) {
		// Not forbidden, but the contract only promises the link line.
		t.Logf("poster-link also printed a poster line:\n%s", out)
	}
	return link, key
}
