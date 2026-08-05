package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

type focusRequest struct {
	WorkspaceID         string `json:"workspaceId"`
	TaskID              string `json:"taskId"`
	ExternalExecutionID string `json:"externalExecutionId"`
}
type execution struct {
	ID         string `json:"id"`
	TaskID     string `json:"taskId"`
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	HostID     string `json:"hostId"`
	Status     string `json:"status"`
}
type commandRunner interface {
	Run(context.Context, ...string) ([]byte, error)
}
type orcaRunner struct{ executable string }

func (r orcaRunner) Run(ctx context.Context, arguments ...string) ([]byte, error) {
	return exec.CommandContext(ctx, r.executable, arguments...).Output()
}

type bridge struct {
	baleyBase string
	origins   map[string]bool
	client    *http.Client
	runner    commandRunner
}

func main() {
	addr := env("BALEY_ORCA_BRIDGE_ADDR", "127.0.0.1:47831")
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host != "127.0.0.1" {
		log.Fatal("BALEY_ORCA_BRIDGE_ADDR must bind to 127.0.0.1")
	}
	base := strings.TrimRight(env("BALEY_SERVER_URL", "http://127.0.0.1:8080"), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "http" || (parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "localhost" && parsed.Hostname() != "::1") {
		log.Fatal("BALEY_SERVER_URL must be a loopback http URL")
	}
	origins := map[string]bool{}
	for _, value := range strings.Split(env("BALEY_VIEWER_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173"), ",") {
		if value = strings.TrimSpace(value); value != "" {
			origins[value] = true
		}
	}
	b := &bridge{baleyBase: base, origins: origins, client: &http.Client{Timeout: 5 * time.Second}, runner: orcaRunner{executable: env("ORCA_CLI_PATH", "orca")}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("POST /v1/orca/focus", b.focus)
	server := &http.Server{Addr: addr, Handler: b.cors(mux), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	log.Printf("Baley Orca bridge listening on http://%s", addr)
	log.Fatal(server.ListenAndServe())
}

func (b *bridge) focus(w http.ResponseWriter, r *http.Request) {
	var input focusRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.WorkspaceID == "" || input.TaskID == "" || input.ExternalExecutionID == "" {
		writeJSON(w, 400, map[string]string{"outcome": "invalid_request", "message": "valid Baley identifiers are required"})
		return
	}
	executionURL := b.baleyBase + "/v1/workspaces/" + url.PathEscape(input.WorkspaceID) + "/external-executions/" + url.PathEscape(input.ExternalExecutionID)
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, executionURL, nil)
	for _, header := range []string{"Cookie", "Authorization"} {
		if value := r.Header.Get(header); value != "" {
			req.Header.Set(header, value)
		}
	}
	response, err := b.client.Do(req)
	if err != nil {
		writeJSON(w, 503, map[string]string{"outcome": "baley_unavailable", "message": "Baley execution could not be verified"})
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		writeJSON(w, response.StatusCode, map[string]string{"outcome": "forbidden", "message": "Baley execution could not be verified"})
		return
	}
	var current execution
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&current) != nil || current.ID != input.ExternalExecutionID || current.TaskID != input.TaskID || current.Provider != "orca" {
		writeJSON(w, 409, map[string]string{"outcome": "execution_mismatch", "message": "Baley Task and Orca execution do not match"})
		return
	}
	if current.Status == "lost" {
		writeJSON(w, 409, map[string]string{"outcome": "execution_lost", "message": "기존 Orca 작업 연결을 먼저 복구하세요."})
		return
	}
	if current.Status != "active" && current.Status != "review" {
		writeJSON(w, 409, map[string]string{"outcome": "unavailable", "message": "Orca execution is not focusable"})
		return
	}
	if current.HostID != "" && current.HostID != "local" {
		writeJSON(w, 409, map[string]string{"outcome": "different_host", "message": "이 작업은 다른 Orca host에서 실행 중입니다."})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	output, err := b.runner.Run(ctx, "terminal", "list", "--worktree", current.ExternalID, "--limit", "50", "--json")
	if err != nil {
		writeJSON(w, 503, map[string]string{"outcome": "orca_unavailable", "message": "Orca runtime을 확인할 수 없습니다."})
		return
	}
	var listed struct {
		OK     bool `json:"ok"`
		Result struct {
			Terminals []struct {
				Handle       string `json:"handle"`
				Connected    bool   `json:"connected"`
				LastOutputAt int64  `json:"lastOutputAt"`
			} `json:"terminals"`
		} `json:"result"`
	}
	if json.NewDecoder(bytes.NewReader(output)).Decode(&listed) != nil || !listed.OK {
		writeJSON(w, 502, map[string]string{"outcome": "orca_unavailable", "message": "Orca terminal 응답을 해석할 수 없습니다."})
		return
	}
	handle, latest := "", int64(-1)
	for _, terminal := range listed.Result.Terminals {
		if terminal.Connected && terminal.Handle != "" && terminal.LastOutputAt >= latest {
			handle, latest = terminal.Handle, terminal.LastOutputAt
		}
	}
	if handle == "" {
		writeJSON(w, 409, map[string]string{"outcome": "worktree_available_no_terminal", "message": "기존 worktree는 있지만 열린 terminal이 없습니다."})
		return
	}
	if _, err = b.runner.Run(ctx, "terminal", "switch", "--terminal", handle, "--json"); err != nil {
		writeJSON(w, 502, map[string]string{"outcome": "focus_failed", "message": "Orca terminal 전환에 실패했습니다."})
		return
	}
	writeJSON(w, 200, map[string]string{"outcome": "focused"})
}

func (b *bridge) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
		if origin == "" || !b.origins[origin] {
			writeJSON(w, 403, map[string]string{"outcome": "forbidden", "message": "origin is not allowed"})
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
