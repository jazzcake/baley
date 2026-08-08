package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/runtimeconfig"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
	"golang.org/x/term"
)

const expectedSchemaVersion int64 = 17

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildTime    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: baley-server migrate [up|down] | account-bootstrap WORKSPACE_ID ACTOR_ID LOGIN_ID DISPLAY_NAME | serve")
	}
	environment := os.Getenv("BALEY_ENV")
	dbURL, configured, err := runtimeconfig.Load("BALEY_DATABASE_URL")
	if err != nil {
		log.Fatal(err)
	}
	if !configured {
		if !isDevelopmentEnvironment(environment) {
			log.Fatal("BALEY_DATABASE_URL or BALEY_DATABASE_URL_FILE is required outside development and test")
		}
		dbURL = "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable"
	}
	if os.Args[1] == "migrate" {
		direction := "up"
		if len(os.Args) > 2 {
			direction = os.Args[2]
		}
		dir := env("BALEY_MIGRATIONS_DIR", filepath.Join("migrations"))
		if err := postgres.Migrate(dbURL, dir, direction); err != nil {
			log.Fatal(err)
		}
		return
	}
	if os.Args[1] != "serve" && os.Args[1] != "account-bootstrap" {
		log.Fatal("unknown command")
	}
	var runtimeConfig runtimeConfig
	var origins []string
	if os.Args[1] == "serve" {
		runtimeConfig, err = resolveRuntimeConfig(environment, os.Getenv("BALEY_AUTH_MODE"), os.Getenv("BALEY_COOKIE_SECURE"))
		if err != nil {
			log.Fatal(err)
		}
		origins, err = resolveViewerOrigins(environment)
		if err != nil {
			log.Fatal(err)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	repo, err := postgres.Open(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Pool.Close()
	if os.Args[1] == "account-bootstrap" {
		if len(os.Args) < 6 {
			log.Fatal("usage: baley-server account-bootstrap WORKSPACE_ID ACTOR_ID LOGIN_ID DISPLAY_NAME")
		}
		stdinReader := bufio.NewReader(os.Stdin)
		password, passwordErr := readPassword("Password (read from stdin): ", stdinReader)
		if passwordErr != nil {
			log.Fatal(passwordErr)
		}
		confirmation, confirmationErr := readPassword("Confirm password (read from stdin): ", stdinReader)
		if confirmationErr != nil || confirmation != password {
			log.Fatal("password confirmation mismatch")
		}
		confirmation = ""
		normalized, normalizeErr := authn.NormalizeLogin(os.Args[4])
		if normalizeErr != nil {
			log.Fatal(normalizeErr)
		}
		passwordPHC, hashErr := (authn.PasswordHasher{}).Hash(password)
		password = ""
		if hashErr != nil {
			log.Fatal(hashErr)
		}
		accountID, idErr := randomUUID()
		if idErr != nil {
			log.Fatal(idErr)
		}
		if err = repo.BootstrapOwner(ctx, os.Args[2], accountID, os.Args[3], os.Args[4], normalized, strings.Join(os.Args[5:], " "), passwordPHC); err != nil {
			log.Fatal(err)
		}
		log.Printf("bootstrapped local Owner account %s for Workspace %s", accountID, os.Args[2])
		return
	}
	if runtimeConfig.AuthMode == "enforced" {
		if err = repo.ValidateEnforcedOwners(ctx); err != nil {
			log.Fatal(err)
		}
	}
	authService, err := authn.NewService(repo)
	if err != nil {
		log.Fatal(err)
	}
	addr := env("BALEY_HTTP_ADDR", "127.0.0.1:8080")
	host, _, err := net.SplitHostPort(addr)
	isLoopback := host == "127.0.0.1" || host == "localhost" || host == "::1"
	allowContainerBind := env("BALEY_ALLOW_CONTAINER_BIND", "false") == "true"
	if err != nil || (!isLoopback && !(allowContainerBind && host == "0.0.0.0")) {
		log.Fatal("BALEY_HTTP_ADDR must bind to loopback")
	}
	service := application.NewService(repo)
	if _, err = service.InterruptExpiredRuns(ctx); err != nil {
		log.Printf("initial Run lease sweep failed: %v", err)
	}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, sweepErr := service.InterruptExpiredRuns(ctx); sweepErr != nil {
					log.Printf("Run lease sweep failed: %v", sweepErr)
				}
			}
		}
	}()
	api := &httpapi.API{
		Service: service, Repo: repo, AllowedOrigins: origins, Auth: authService,
		AuthMode: runtimeConfig.AuthMode, CookieSecure: runtimeConfig.CookieSecure,
		Build: httpapi.BuildInfo{Version: buildVersion, Commit: buildCommit, BuiltAt: buildTime, SchemaVersion: expectedSchemaVersion},
		ReadyCheck: func(readyCtx context.Context) (int64, error) {
			return repo.Readiness(readyCtx, expectedSchemaVersion)
		},
	}
	server := &http.Server{
		Addr: addr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("Baley server %s (%s) listening on http://%s", buildVersion, buildCommit, addr)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func readPassword(prompt string, reader *bufio.Reader) (string, error) {
	log.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		_, _ = fmt.Fprintln(os.Stderr)
		return string(raw), err
	}
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		return "", err
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func randomUUID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type runtimeConfig struct {
	AuthMode     string
	CookieSecure bool
}

func resolveRuntimeConfig(environment, requestedAuthMode, requestedCookieSecure string) (runtimeConfig, error) {
	environment = strings.ToLower(strings.TrimSpace(environment))
	development := isDevelopmentEnvironment(environment)

	authMode := strings.ToLower(strings.TrimSpace(requestedAuthMode))
	if authMode == "" {
		if development {
			authMode = "legacy"
		} else {
			authMode = "enforced"
		}
	}
	if authMode != "legacy" && authMode != "enforced" {
		return runtimeConfig{}, errors.New("BALEY_AUTH_MODE must be legacy or enforced")
	}
	if !development && authMode == "legacy" {
		return runtimeConfig{}, errors.New("BALEY_AUTH_MODE=legacy is forbidden outside development and test environments")
	}

	cookieSecure := !development
	if raw := strings.TrimSpace(requestedCookieSecure); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return runtimeConfig{}, errors.New("BALEY_COOKIE_SECURE must be a boolean")
		}
		cookieSecure = value
	}
	if !development && !cookieSecure {
		return runtimeConfig{}, errors.New("BALEY_COOKIE_SECURE=false is forbidden outside development and test environments")
	}
	return runtimeConfig{AuthMode: authMode, CookieSecure: cookieSecure}, nil
}

func isDevelopmentEnvironment(environment string) bool {
	environment = strings.ToLower(strings.TrimSpace(environment))
	return environment == "" || environment == "development" || environment == "dev" || environment == "test" || environment == "local"
}

func resolveViewerOrigins(environment string) ([]string, error) {
	raw := os.Getenv("BALEY_VIEWER_ORIGINS")
	if raw == "" {
		raw = os.Getenv("BALEY_VIEWER_ORIGIN")
	}
	if raw == "" {
		if !isDevelopmentEnvironment(environment) {
			return nil, errors.New("BALEY_VIEWER_ORIGINS is required outside development and test")
		}
		raw = "http://127.0.0.1:5173,http://localhost:5173"
	}
	origins := make([]string, 0)
	seen := map[string]bool{}
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
			(parsed.Path != "" && parsed.Path != "/") {
			return nil, fmt.Errorf("invalid viewer origin %q", origin)
		}
		if !isDevelopmentEnvironment(environment) && parsed.Scheme != "https" {
			return nil, fmt.Errorf("viewer origin %q must use https outside development and test", origin)
		}
		origin = strings.TrimSuffix(origin, "/")
		if !seen[origin] {
			seen[origin] = true
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return nil, errors.New("at least one viewer origin is required")
	}
	return origins, nil
}
