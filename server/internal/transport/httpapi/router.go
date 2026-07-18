package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

type API struct {
	Service       *application.Service
	Repo          *postgres.Repository
	AllowedOrigin string
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}", a.workspace)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/graph", a.graph)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/tasks/{publicId}", a.task)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/gates/{gateId}/status", a.gate)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/decisions", a.decisions)
	mux.HandleFunc("GET /v1/workspaces/{workspaceId}/events", a.events)
	mux.HandleFunc("POST /v1/commands/preview", a.preview)
	mux.HandleFunc("POST /v1/commands/execute", a.execute)
	return a.cors(mux)
}

func (a *API) workspace(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, s.Workspace)
}
func (a *API) graph(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"workspace": s.Workspace, "phases": s.Phases, "lanes": s.Lanes, "tasks": s.Tasks, "dependencies": s.Dependencies, "gates": s.Gates, "decisions": projectDecisions(s)})
}
func (a *API) task(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("publicId"))
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "publicId must be an integer"}})
		return
	}
	v, err := a.Repo.Task(r.Context(), r.PathValue("workspaceId"), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) gate(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	for _, g := range s.Gates {
		if g.ID == r.PathValue("gateId") {
			writeJSON(w, 200, g)
			return
		}
	}
	writeJSON(w, 404, map[string]any{"error": map[string]string{"code": "not_found", "message": "gate not found"}})
}
func (a *API) decisions(w http.ResponseWriter, r *http.Request) {
	s, err := a.Repo.LoadSnapshot(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, projectDecisions(s))
}
func (a *API) events(w http.ResponseWriter, r *http.Request) {
	v, err := a.Repo.Events(r.Context(), r.PathValue("workspaceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) preview(w http.ResponseWriter, r *http.Request) {
	var req application.CommandRequest
	if !decode(w, r, &req) {
		return
	}
	v, err := a.Service.Preview(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}
func (a *API) execute(w http.ResponseWriter, r *http.Request) {
	var req application.CommandRequest
	if !decode(w, r, &req) {
		return
	}
	v, err := a.Service.Execute(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func projectDecisions(s application.Snapshot) []map[string]any {
	out := []map[string]any{}
	for _, t := range s.Tasks {
		if t.Status == "implemented" {
			out = append(out, map[string]any{"action": "task.confirm", "entityType": "task", "entityId": t.PublicID, "expectedWorkspaceRevision": s.Workspace.Revision})
		}
	}
	for _, g := range s.Gates {
		if g.Status == "ready" {
			out = append(out, map[string]any{"action": "gate.pass", "entityType": "gate", "entityId": g.ID, "expectedWorkspaceRevision": s.Workspace.Revision, "decisionSnapshotHash": application.DecisionSnapshotHash(s, g)})
		}
	}
	return out
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		writeJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": err.Error()}})
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	var ce *application.CommandError
	if !errors.As(err, &ce) {
		writeJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "internal server error"}})
		return
	}
	status := 422
	switch ce.Code {
	case "not_found":
		status = 404
	case "stale_revision", "idempotency_conflict", "invalid_state_transition", "gate_not_ready", "gate_not_current":
		status = 409
	case "invalid_request":
		status = 400
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": ce.Code, "message": ce.Message}})
}
func (a *API) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && origin == a.AllowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		}
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
