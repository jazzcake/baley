package integration_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/pressly/goose/v3"
)

const (
	task183Workspace = "00000000-0000-4000-8000-000000000001"
	task183ID        = "0393bf4a-c53d-4ccd-937c-50528b044c32"
)

type historicalFixture struct {
	Snapshot struct {
		WorkspaceID  string `json:"workspaceId"`
		TaskID       string `json:"taskId"`
		TaskPublicID int    `json:"taskPublicId"`
	} `json:"snapshot"`
	Events []struct {
		ID      string `json:"id"`
		Command struct {
			ID                 string `json:"id"`
			IdempotencyKey     string `json:"idempotencyKey"`
			Name               string `json:"name"`
			Hash               string `json:"hash"`
			RequestFingerprint string `json:"requestFingerprint"`
		} `json:"command"`
		EventType          string          `json:"eventType"`
		EntityType         string          `json:"entityType"`
		EntityID           string          `json:"entityId"`
		WorkspaceRevision  int64           `json:"workspaceRevision"`
		InitiatedByActorID string          `json:"initiatedByActorId"`
		ExecutedByActorID  string          `json:"executedByActorId"`
		ApprovedByActorID  *string         `json:"approvedByActorId"`
		CreatedAt          time.Time       `json:"createdAt"`
		Payload            json.RawMessage `json:"payload"`
	} `json:"events"`
	ApprovalAttestations []struct {
		ID                  string    `json:"id"`
		ApprovedByActorID   string    `json:"approvedByActorId"`
		ApprovedCommandHash string    `json:"approvedCommandHash"`
		Action              string    `json:"action"`
		EntityType          string    `json:"entityType"`
		EntityID            string    `json:"entityId"`
		WorkspaceRevision   int64     `json:"workspaceRevision"`
		ExecutedCommandID   string    `json:"executedCommandId"`
		RecordedAt          time.Time `json:"recordedAt"`
	} `json:"approvalAttestations"`
}

func TestMigration27HistoricalTaskJournalFixture(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	pool := migration27Pool(t, ctx, url)
	defer pool.Close()
	migrations := filepath.Join("..", "migrations")
	fixture := readHistoricalFixture(t)
	prepareMigration27Schema25(t, ctx, pool, url, migrations)
	seedHistoricalFixture(t, ctx, pool, fixture)
	migrateUpTo(t, url, migrations, 26)
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatalf("migrate 25 to 27: %v", err)
	}

	var version, count int
	if err := pool.QueryRow(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&version); err != nil || version != 27 {
		t.Fatalf("schema version=%d err=%v", version, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM task_journal_entries WHERE workspace_id=$1 AND task_id=$2", task183Workspace, task183ID).Scan(&count); err != nil || count != 4 {
		t.Fatalf("Task #183 journal count=%d err=%v", count, err)
	}

	type oracle struct{ stage, event, narrative string }
	for _, expected := range []oracle{
		{"created", "7e40ccb3-a82d-46c8-8bce-eabd02c092b1", ""},
		{"run_started", "1d49006d-4311-45c0-a56f-fd6a900aeaf3", ""},
		{"implemented", "74bb1376-7299-4107-9fdf-3347237cb081", "nonempty"},
		{"confirmed", "320bf0d1-618e-4e8f-aae4-9cc13edc15d1", ""},
	} {
		var stage, narrative string
		var eventID string
		if err := pool.QueryRow(ctx, `SELECT lifecycle_stage,event_id,COALESCE(narrative,'') FROM task_journal_entries WHERE id=$1`, expected.event).Scan(&stage, &eventID, &narrative); err != nil {
			t.Fatal(err)
		}
		if stage != expected.stage || eventID != expected.event || (expected.narrative == "" && narrative != "") || (expected.narrative == "nonempty" && narrative == "") {
			t.Fatalf("oracle %s: stage=%q event=%q narrative=%q", expected.event, stage, eventID, narrative)
		}
	}

	var runKind, runClientID, backfillSource, narrativeState string
	if err := pool.QueryRow(ctx, `SELECT context->>'kind',context->>'clientRunId',context#>>'{_backfill,source}',context#>>'{_backfill,narrativeState}' FROM task_journal_entries WHERE id='1d49006d-4311-45c0-a56f-fd6a900aeaf3'`).Scan(&runKind, &runClientID, &backfillSource, &narrativeState); err != nil {
		t.Fatal(err)
	}
	if runKind != "detailed_planning" || runClientID != "ab77f6a3-7583-4cd4-a50b-299462580960" || backfillSource != "historical_event" || narrativeState != "not_recorded" {
		t.Fatalf("run projection kind=%q client=%q source=%q narrative=%q", runKind, runClientID, backfillSource, narrativeState)
	}

	var assessment, narrative string
	if err := pool.QueryRow(ctx, `SELECT context->>'assessment',narrative FROM task_journal_entries WHERE id='74bb1376-7299-4107-9fdf-3347237cb081'`).Scan(&assessment, &narrative); err != nil || assessment != narrative {
		t.Fatalf("implemented assessment differs from narrative: equal=%v err=%v", assessment == narrative, err)
	}
	var approver *string
	if err := pool.QueryRow(ctx, `SELECT approved_by_actor_id FROM task_journal_entries WHERE id='320bf0d1-618e-4e8f-aae4-9cc13edc15d1'`).Scan(&approver); err != nil || approver == nil || *approver != "00000000-0000-4000-8000-000000000002" {
		t.Fatalf("confirmed approver=%v err=%v", approver, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_journal_entries j JOIN human_approval_attestations a ON a.workspace_id=j.workspace_id AND a.executed_command_id=j.command_id AND a.approved_by_actor_id=j.approved_by_actor_id WHERE j.id='320bf0d1-618e-4e8f-aae4-9cc13edc15d1'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("confirmed approval attestation links=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_journal_entries WHERE event_id IN ('e21079ed-3b77-4af4-86fd-d395eba8d9b5','00ef65c1-8328-45f2-ae34-539b96c214a3','b3240903-2ea2-4a1d-94df-55dc3c69835d')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("excluded lifecycle rows=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_journal_entries WHERE context ?| array['goal','whyNow','alternatives','completionContract','outcome']`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invented context fields=%d err=%v", count, err)
	}

	// Paired cursor ordering and Workspace isolation are properties of the same
	// repository query used by HTTP, Viewer, and MCP adapters.
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "migration-27-query-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	page, err := repo.TaskJournal(ctx, task183Workspace, 183, time.Time{}, "", 2)
	if err != nil || len(page) != 2 {
		t.Fatalf("first journal page=%d err=%v", len(page), err)
	}
	next, err := repo.TaskJournal(ctx, task183Workspace, 183, page[1].RecordedAt, page[1].ID, 2)
	if err != nil || len(next) != 2 || page[1].RecordedAt.Before(next[0].RecordedAt) {
		t.Fatalf("second journal page=%d err=%v", len(next), err)
	}
	foreign, err := repo.TaskJournal(ctx, "another-workspace", 0, time.Time{}, "", 100)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("cross-Workspace rows=%d err=%v", len(foreign), err)
	}

	// Down is intentionally a no-op for rows; a second Up must validate and
	// reuse the byte-equivalent projections.
	if err = postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM task_journal_entries WHERE workspace_id=$1", task183Workspace).Scan(&count); err != nil || count != 4 {
		t.Fatalf("idempotent replay count=%d err=%v", count, err)
	}
}

func TestMigration27FailsClosedAndRollsBack(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	pool := migration27Pool(t, ctx, url)
	defer pool.Close()
	migrations := filepath.Join("..", "migrations")
	fixture := readHistoricalFixture(t)

	tests := []struct {
		name, mutation string
		retainedRows   int
	}{
		{"missing task", `UPDATE events SET payload=jsonb_set(payload,'{taskId}','"missing-task"') WHERE event_type='run.started'`, 0},
		{"missing executor", `UPDATE events SET executed_by_actor_id=NULL WHERE event_type='run.started'`, 0},
		{"entity mismatch", `UPDATE events SET entity_id='wrong-run' WHERE event_type='run.started'`, 0},
		{"malformed payload", `UPDATE events SET payload=jsonb_set(payload,'{kind}','[]'::jsonb) WHERE event_type='run.started'`, 0},
		{"duplicate command", `UPDATE events SET command_id='bb03c8c6-6dd3-43bd-944b-210e145f77c4',command_event_index=2 WHERE event_type='task.implemented_reported'`, 0},
		{"conflicting existing row", `INSERT INTO task_journal_entries(id,workspace_id,task_id,event_id,command_id,lifecycle_stage,narrative,schema_version,context,initiated_by_actor_id,executed_by_actor_id,occurred_at,recorded_at) SELECT e.id,e.workspace_id,'` + task183ID + `',e.id,e.command_id,'created','mismatch',1,'{}'::jsonb,e.initiated_by_actor_id,e.executed_by_actor_id,e.created_at,e.created_at FROM events e WHERE e.id='7e40ccb3-a82d-46c8-8bce-eabd02c092b1'`, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepareMigration27Schema25(t, ctx, pool, url, migrations)
			seedHistoricalFixture(t, ctx, pool, fixture)
			migrateUpTo(t, url, migrations, 26)
			if _, err := pool.Exec(ctx, tt.mutation); err != nil {
				t.Fatal(err)
			}
			if err := postgres.Migrate(url, migrations, "up"); err == nil {
				t.Fatal("migration 27 accepted invalid historical evidence")
			}
			var version, rows int
			if err := pool.QueryRow(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&version); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM task_journal_entries").Scan(&rows); err != nil {
				t.Fatal(err)
			}
			if version != 26 || rows != tt.retainedRows {
				t.Fatalf("partial failure leaked: version=%d rows=%d wantRows=%d", version, rows, tt.retainedRows)
			}
		})
	}
	cleanLatestMigration27Data(t, ctx, pool)
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatalf("restore latest schema after fail-closed cases: %v", err)
	}
}

func migrateUpTo(t *testing.T, url, migrations string, version int64) {
	t.Helper()
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, migrations, version); err != nil {
		t.Fatal(err)
	}
}

func migration27Pool(t *testing.T, ctx context.Context, url string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

func readHistoricalFixture(t *testing.T) historicalFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "task_journal_history_task_183.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture historicalFixture
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Snapshot.WorkspaceID != task183Workspace || fixture.Snapshot.TaskID != task183ID || fixture.Snapshot.TaskPublicID != 183 || len(fixture.Events) != 7 || len(fixture.ApprovalAttestations) != 1 {
		t.Fatalf("unexpected #183 fixture identity or Event count: %+v events=%d", fixture.Snapshot, len(fixture.Events))
	}
	return fixture
}

func prepareMigration27Schema25(t *testing.T, ctx context.Context, pool *pgxpool.Pool, url, migrations string) {
	t.Helper()
	// A preceding fail-closed case intentionally leaves Goose at 26 with its
	// invalid source Event intact. Remove disposable data before asking Goose to
	// apply 27 again.
	cleanLatestMigration27Data(t, ctx, pool)
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	cleanLatestMigration27Data(t, ctx, pool)
	var version int
	for {
		if err := pool.QueryRow(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&version); err != nil {
			t.Fatal(err)
		}
		if version <= 25 {
			break
		}
		if err := postgres.Migrate(url, migrations, "down"); err != nil {
			t.Fatal(err)
		}
	}
	if version != 25 {
		t.Fatalf("prepared schema version=%d", version)
	}
}

func cleanLatestMigration27Data(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var events, journal *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('events')::text,to_regclass('task_journal_entries')::text").Scan(&events, &journal); err != nil {
		t.Fatal(err)
	}
	if events == nil {
		return
	}
	if journal != nil {
		if _, err := pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE task_journal_entries,events,human_approval_attestations,commands,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err := pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
			t.Fatal(err)
		}
	}
}

func seedHistoricalFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture historicalFixture) {
	t.Helper()
	statements := []string{
		`INSERT INTO actors(id,display_name,actor_type) VALUES ('00000000-0000-4000-8000-000000000002','Fixture Owner','human'),('00000000-0000-4000-8000-000000000003','Fixture Agent','agent')`,
		`INSERT INTO workspaces(id,name,state,revision) VALUES ('` + task183Workspace + `','Task Journal Fixture','draft',1354)`,
		`INSERT INTO phases(workspace_id,id,name,position,state) VALUES ('` + task183Workspace + `','multi-user-operations','Multi-user operations',0,'active')`,
		`INSERT INTO lanes(workspace_id,id,name,state) VALUES ('` + task183Workspace + `','server','Server','active')`,
		`INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,title,status) VALUES ('` + task183Workspace + `','` + task183ID + `',183,'server','multi-user-operations','Task #183','confirmed')`,
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	seenCommand := map[string]bool{}
	commandIndexes := map[string]int{}
	for _, event := range fixture.Events {
		if !seenCommand[event.Command.ID] {
			_, err := pool.Exec(ctx, `INSERT INTO commands(id,workspace_id,idempotency_key,command_name,command_hash,request_fingerprint,workspace_revision,result,created_at,initiated_by_actor_id,executed_by_actor_id) VALUES ($1,$2,$3,$4,$5,$6,$7,'{}'::jsonb,$8,$9,$10)`, event.Command.ID, task183Workspace, event.Command.IdempotencyKey, event.Command.Name, event.Command.Hash, event.Command.RequestFingerprint, event.WorkspaceRevision, event.CreatedAt, event.InitiatedByActorID, event.ExecutedByActorID)
			if err != nil {
				t.Fatal(err)
			}
			seenCommand[event.Command.ID] = true
		}
		index := commandIndexes[event.Command.ID]
		_, err := pool.Exec(ctx, `INSERT INTO events(id,workspace_id,command_id,workspace_revision,event_type,payload,created_at,command_event_index,entity_type,entity_id,initiated_by_actor_id,executed_by_actor_id,approved_by_actor_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, event.ID, task183Workspace, event.Command.ID, event.WorkspaceRevision, event.EventType, []byte(event.Payload), event.CreatedAt, index, event.EntityType, event.EntityID, event.InitiatedByActorID, event.ExecutedByActorID, event.ApprovedByActorID)
		if err != nil {
			t.Fatal(err)
		}
		commandIndexes[event.Command.ID] = index + 1
	}
	for _, approval := range fixture.ApprovalAttestations {
		_, err := pool.Exec(ctx, `INSERT INTO human_approval_attestations(id,workspace_id,approved_by_actor_id,approved_command_hash,action,entity_type,entity_id,workspace_revision,executed_command_id,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, approval.ID, task183Workspace, approval.ApprovedByActorID, approval.ApprovedCommandHash, approval.Action, approval.EntityType, approval.EntityID, approval.WorkspaceRevision, approval.ExecutedCommandID, approval.RecordedAt)
		if err != nil {
			t.Fatal(err)
		}
	}
}
