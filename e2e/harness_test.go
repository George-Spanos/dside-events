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

// server is one running `dside-events serve` process with its own SQLite
// file and DEV_OTP_FILE.
type server struct {
	dir     string
	dbPath  string
	otpFile string
	url     string // http://127.0.0.1:PORT, no trailing slash

	cmd  *exec.Cmd
	done chan struct{}

	mu     sync.Mutex
	log    bytes.Buffer
	dumped int
}

// serverOpts configures a per-test server.
type serverOpts struct {
	// env adds or overrides environment variables, e.g. OTP_TTL, OTP_MAX_ATTEMPTS.
	env map[string]string
}

var listeningLine = regexp.MustCompile(`^listening on (http://127\.0\.0\.1:\d+)\s*$`)

// startServer launches a fresh server for one test and stops it on cleanup.
// Its log is attached to the test output when the test fails.
func startServer(t *testing.T, opts serverOpts) *server {
	t.Helper()
	dir, err := os.MkdirTemp("", "dside-e2e-test-")
	if err != nil {
		t.Fatal(err)
	}
	s, err := launchServer(dir, opts.env)
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

// launchServer starts `serve` in dir and waits for the listening line and a
// healthy /healthz.
func launchServer(dir string, env map[string]string) (*server, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &server{
		dir:     dir,
		dbPath:  filepath.Join(dir, "events.db"),
		otpFile: filepath.Join(dir, "otp.log"),
		done:    make(chan struct{}),
	}
	cmd := exec.Command(binPath, "serve")
	cmd.Env = append(os.Environ(),
		"ADDR=127.0.0.1:0",
		"DB_PATH="+s.dbPath,
		"DEV_OTP_FILE="+s.otpFile,
		"SMTP_HOST=",
		"LOG_LEVEL=debug",
	)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
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

var posterLine = regexp.MustCompile(`(?m)^poster (\S+) (\S+)\s*$`)

// runAddPoster runs `dside-events add-poster` against dbPath and parses
// `poster <slug> <email>` from its stdout.
func runAddPoster(dbPath, email, name string) (poster, error) {
	cmd := exec.Command(binPath, "add-poster", "-email", email, "-name", name)
	cmd.Env = append(os.Environ(), "DB_PATH="+dbPath, "LOG_LEVEL=warn")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return poster{}, fmt.Errorf("add-poster %s: %v\nstdout: %s\nstderr: %s", email, err, stdout.String(), stderr.String())
	}
	m := posterLine.FindStringSubmatch(stdout.String())
	if m == nil {
		return poster{}, fmt.Errorf("add-poster %s: stdout lacks 'poster <slug> <email>':\n%s", email, stdout.String())
	}
	return poster{Email: m[2], Name: name, Slug: m[1]}, nil
}

// addPoster is runAddPoster for use inside a test, against s's database.
func addPoster(t testing.TB, s *server, email, name string) poster {
	t.Helper()
	p, err := runAddPoster(s.dbPath, email, name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// otpCodes returns every code the DEV_OTP_FILE holds for email, oldest first.
// Lines are `<RFC3339>\t<email>\t<code>`.
func (s *server) otpCodes(email string) []string {
	data, err := os.ReadFile(s.otpFile)
	if err != nil {
		return nil
	}
	var codes []string
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) == 3 && f[1] == email {
			codes = append(codes, f[2])
		}
	}
	return codes
}

// waitOTP polls until at least minCount codes exist for email and returns the
// most recent one.
func waitOTP(t testing.TB, s *server, email string, minCount int) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		codes := s.otpCodes(email)
		if len(codes) >= minCount {
			return codes[len(codes)-1]
		}
		if time.Now().After(deadline) {
			t.Fatalf("no OTP line for %s in %s (have %d, want %d)", email, s.otpFile, len(codes), minCount)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// readOTP returns the last code delivered to email (waiting for at least one).
func readOTP(t testing.TB, s *server, email string) string {
	t.Helper()
	return waitOTP(t, s, email, 1)
}

// otpLineCount is how many codes have been delivered to email so far.
func otpLineCount(s *server, email string) int {
	return len(s.otpCodes(email))
}
