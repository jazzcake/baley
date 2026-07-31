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
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: baley-server migrate [up|down] | account-bootstrap WORKSPACE_ID ACTOR_ID LOGIN_ID DISPLAY_NAME | serve")
	}
	dbURL := env("BALEY_DATABASE_URL", "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable")
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
	runtimeConfig, err := resolveRuntimeConfig(os.Getenv("BALEY_ENV"), os.Getenv("BALEY_AUTH_MODE"), os.Getenv("BALEY_COOKIE_SECURE"))
	if err != nil {
		log.Fatal(err)
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
	if err != nil || !(host == "127.0.0.1" || host == "localhost" || host == "::1") {
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
	api := &httpapi.API{Service: service, Repo: repo, AllowedOrigins: viewerOrigins(), Auth: authService, AuthMode: runtimeConfig.AuthMode, CookieSecure: runtimeConfig.CookieSecure}
	server := &http.Server{Addr: addr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("Baley server listening on http://%s", addr)
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
	development := environment == "" || environment == "development" || environment == "dev" || environment == "test" || environment == "local"

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

func viewerOrigins() []string {
	raw := os.Getenv("BALEY_VIEWER_ORIGINS")
	if raw == "" {
		raw = os.Getenv("BALEY_VIEWER_ORIGIN")
	}
	if raw == "" {
		raw = "http://127.0.0.1:5173,http://localhost:5173"
	}
	origins := make([]string, 0)
	for _, origin := range strings.Split(raw, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}
