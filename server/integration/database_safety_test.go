package integration_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var disposableDatabaseName = regexp.MustCompile(`(?i)(^|_)(test|testing)(_|$)`)

func requireDisposableDatabase(t *testing.T, rawURL string) {
	t.Helper()
	if err := validateDisposableDatabaseURL(rawURL); err != nil {
		t.Fatalf("refusing destructive integration test database: %v", err)
	}
}

func validateDisposableDatabaseURL(rawURL string) error {
	config, err := pgx.ParseConfig(rawURL)
	if err != nil {
		return err
	}
	host := config.Host
	if host != "localhost" && net.ParseIP(host) == nil {
		return fmt.Errorf("host %q is not loopback", host)
	}
	if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
		return fmt.Errorf("host %q is not loopback", host)
	}
	database := config.Database
	if !disposableDatabaseName.MatchString(database) {
		return fmt.Errorf("database %q must contain a standalone test/testing marker", database)
	}
	return nil
}

func TestValidateDisposableDatabaseURL(t *testing.T) {
	for _, rawURL := range []string{
		"postgres://baley:baley@127.0.0.1:54329/baley_test?sslmode=disable",
		"postgres://baley:baley@localhost:54329/baley_test_121?sslmode=disable",
		"postgres://baley:baley@[::1]:54329/testing_baley?sslmode=disable",
	} {
		if err := validateDisposableDatabaseURL(rawURL); err != nil {
			t.Errorf("safe URL %q rejected: %v", rawURL, err)
		}
	}
	for _, rawURL := range []string{
		"postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable",
		"postgres://baley:baley@db.internal:5432/baley_test?sslmode=disable",
		"postgres://baley:baley@10.0.0.8:5432/baley_test?sslmode=disable",
		"postgres://baley:baley@127.0.0.1:54329/baley_reconstructed_20260726?sslmode=disable",
		"postgres://baley:baley@127.0.0.1:54329/baley_test?host=prod.example.com&database=prod",
	} {
		if err := validateDisposableDatabaseURL(rawURL); err == nil {
			t.Errorf("unsafe URL %q accepted", rawURL)
		}
	}
}

func TestValidateDisposableDatabaseConnection(t *testing.T) {
	rawURL := os.Getenv("BALEY_TEST_DATABASE_URL")
	if rawURL == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, rawURL)
	pool, err := pgxpool.New(context.Background(), rawURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var database, serverAddress, clientAddress string
	if err = pool.QueryRow(context.Background(), "SELECT current_database(),inet_server_addr()::text,inet_client_addr()::text").Scan(&database, &serverAddress, &clientAddress); err != nil {
		t.Fatal(err)
	}
	parseAddress := func(value string) net.IP {
		if ip := net.ParseIP(value); ip != nil {
			return ip
		}
		ip, _, _ := net.ParseCIDR(value)
		return ip
	}
	serverIP, clientIP := parseAddress(serverAddress), parseAddress(clientAddress)
	// A loopback-published Docker PostgreSQL port terminates on a private bridge
	// address. Both ends must still be loopback/private, and the parsed client
	// config above must independently resolve to a literal loopback target.
	if !disposableDatabaseName.MatchString(database) || serverIP == nil || clientIP == nil ||
		!(serverIP.IsLoopback() || serverIP.IsPrivate()) || !(clientIP.IsLoopback() || clientIP.IsPrivate()) {
		t.Fatalf("effective database connection is not disposable local: database=%q server=%q client=%q", database, serverAddress, clientAddress)
	}
}
