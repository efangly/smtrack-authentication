//go:build e2e

// Package e2e drives the real service (real Postgres and Redis) over HTTP. It
// is not part of `go test ./...` — run it explicitly with:
//
//	go test -tags=e2e ./test/e2e/... -v
//
// The suite needs:
//
//   - The service's environment (DATABASE_URL, JWT_SECRET, JWT_REFRESH_SECRET,
//     REDIS_HOST, ...) in the process environment or the repo's .env. Use an
//     empty throwaway database: the suite creates tenants, departments and
//     users there, and audit_logs rows can never be deleted.
//
//   - E2E_ADMIN_USERNAME / E2E_ADMIN_PASSWORD naming an existing SUPER
//     account (it creates tenants, hidden tenants and other admins, and resets
//     passwords). Public registration only ever creates a GUEST, so seed it
//     once, after the service has migrated the database:
//
//     INSERT INTO tenants (id, name, kind, hidden) VALUES ('e2e-root', 'E2E root', 'OTHER', true);
//     INSERT INTO users (id, tenant_id, username, password, role)
//     VALUES ('e2e-super', 'e2e-root', 'e2e-super', '<bcrypt hash>', 'SUPER');
//
//   - Optionally E2E_BASE_URL (for example http://localhost:8080/auth) to
//     test an already-running instance instead of spawning one.
//
// /auth/login and /auth/register share a 10-per-minute limit per client, so the
// suite paces itself and takes several minutes.
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
		parent := filepath.Dir(dir)
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
