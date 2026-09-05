// Package e2e is the black-box end-to-end suite for dside-events.
//
// It builds the real binary, starts `serve` on a random port, seeds two
// curators with `add-poster` (which prints their secret login links) and
// drives the server over plain HTTP exactly the way a browser without
// JavaScript would. Nothing in here imports the server.
//
// Every test carries a `// spec: Name[, Name…]` header naming the rules,
// surfaces and invariants of spec/dside-events.allium it exercises.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	_ "time/tzdata" // Europe/Athens must resolve even on hosts without tzdata.
)

// poster is a curator seeded through the CLI. Link is the secret login link
// `add-poster` printed (`<BASE_URL>/k/<key>`); Key is its 43-character key.
type poster struct {
	Name, Slug, Link, Key string
}

var (
	binPath string  // the built server binary
	shared  *server // the server every test uses unless it starts its own
	poster1 poster  // Maria P.
	poster2 poster  // Nikos K.
)

func TestMain(m *testing.M) {
	code, err := runSuite(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e harness:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func runSuite(m *testing.M) (int, error) {
	root, err := filepath.Abs("..")
	if err != nil {
		return 1, err
	}
	dir, err := os.MkdirTemp("", "dside-e2e-")
	if err != nil {
		return 1, err
	}
	defer os.RemoveAll(dir)

	binPath, err = buildBinary(root, dir)
	if err != nil {
		return 1, err
	}

	// The server starts first: add-poster prints links against BASE_URL, and
	// those links must open on the running server.
	s, err := launchServer(filepath.Join(dir, "shared"))
	if err != nil {
		return 1, err
	}
	shared = s
	if poster1, err = runAddPoster(s, "Maria P.", ""); err != nil {
		s.stop()
		return 1, err
	}
	if poster2, err = runAddPoster(s, "Nikos K.", ""); err != nil {
		s.stop()
		return 1, err
	}

	code := m.Run()
	s.stop()
	if code != 0 {
		fmt.Fprintf(os.Stderr, "--- shared server log ---\n%s\n--- end ---\n", s.logs())
	}
	return code, nil
}

// buildBinary compiles the server from the module root unless DSIDE_E2E_BIN
// points at a prebuilt binary.
func buildBinary(root, dir string) (string, error) {
	if b := os.Getenv("DSIDE_E2E_BIN"); b != "" {
		if _, err := os.Stat(b); err != nil {
			return "", fmt.Errorf("DSIDE_E2E_BIN: %w", err)
		}
		return b, nil
	}
	out := filepath.Join(dir, "dside-events")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = root
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build failed: %v\n%s", err, strings.TrimSpace(string(b)))
	}
	return out, nil
}
