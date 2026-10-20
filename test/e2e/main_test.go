//go:build e2e

// Package e2e drives the real service (real Postgres/Redis/RabbitMQ from .env)
// over HTTP to verify this round's security and audit-log changes. It is not
// part of `go test ./...` — run it explicitly with:
//
//	go test -tags=e2e ./test/e2e/... -v
//
// It expects E2E_ADMIN_USERNAME / E2E_ADMIN_PASSWORD to name an existing
// SUPER- or ADMIN-tier account in the target database, since public
// registration can only ever create a GUEST. Set E2E_BASE_URL to point at an
// already-running instance instead of spawning one.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

var (
	baseURL       string
	adminUsername string
	adminPassword string
)

func TestMain(m *testing.M) {
	adminUsername = os.Getenv("E2E_ADMIN_USERNAME")
	adminPassword = os.Getenv("E2E_ADMIN_PASSWORD")
	if adminUsername == "" || adminPassword == "" {
		fmt.Println("E2E_ADMIN_USERNAME and E2E_ADMIN_PASSWORD must be set to an existing admin-tier account")
		os.Exit(1)
	}

	baseURL = os.Getenv("E2E_BASE_URL")
	var stop func()
	if baseURL == "" {
		baseURL = "http://localhost:8080/auth"
		var err error
		stop, err = spawnService()
		if err != nil {
			fmt.Println("failed to start service:", err)
			os.Exit(1)
		}
	}

	if err := waitForHealth(30 * time.Second); err != nil {
		if stop != nil {
			stop()
		}
		fmt.Println("service did not become healthy:", err)
		os.Exit(1)
	}

	code := m.Run()
	if stop != nil {
		stop()
	}
	os.Exit(code)
}

// spawnService builds the real binary and runs it directly as a subprocess,
// inheriting the process environment (and therefore the repo's .env once
// main.go loads it) so the suite exercises the exact wiring in main.go
// rather than a test double.
//
// It deliberately does NOT use `go run .`: that spawns the compiled binary
// as a grandchild process, and killing the `go run` wrapper via context
// cancellation does not reach it — the server keeps running, holds the
// port, and orphans itself across test runs.
func spawnService() (func(), error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return nil, err
	}

	binPath := repoRoot + "/test/e2e/.bin-auth-service"
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = repoRoot
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return nil, fmt.Errorf("build service binary: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		os.Remove(binPath)
		return nil, err
	}

	stop := func() {
		cancel()
		_ = cmd.Wait()
		os.Remove(binPath)
	}
	return stop, nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir, nil
		}
		parent := dir + "/.."
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func waitForHealth(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health check returned %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(300 * time.Millisecond)
	}
	return lastErr
}
