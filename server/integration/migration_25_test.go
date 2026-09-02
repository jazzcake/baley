package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration25FailsClosedForPreSessionMCPLinks(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	migrations := filepath.Join("..", "migrations")
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE security_events,agent_tokens,mcp_gateway_registrations,mcp_connection_requests,workspace_memberships,account_sessions,account_credentials,accounts,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO mcp_connection_requests(
		id,workspace_id,agent_actor_id,gateway_id,secret_hash,status,created_at,expires_at,
		linked_by_actor_id,linked_at,consumed_at,login_code_hash,login_code_expires_at,login_actor_id)
		VALUES
		('plain-pending',$1,$2,'gateway-plain',$3,'pending',$4,$5,NULL,NULL,NULL,NULL,NULL,NULL),
		('code-pending',$1,$2,'gateway-code',$3,'pending',$4,$5,NULL,NULL,NULL,$3,$5,$6),
		('legacy-linked',$1,$2,'gateway-linked',$3,'linked',$4,$5,$6,$4,NULL,NULL,NULL,NULL),
		('already-consumed',$1,$2,'gateway-consumed',$3,'consumed',$4,$5,$6,$4,$4,NULL,NULL,NULL)`,
		postgres.DemoWorkspaceID, postgres.DemoAgentActorID, postgres.DigestSecret("migration-25-secret"), now, now.Add(time.Hour), postgres.DemoHumanActorID); err != nil {
		t.Fatal(err)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}

	rows, err := repo.Pool.Query(ctx, `SELECT id,status,login_session_id IS NULL FROM mcp_connection_requests ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var id, status string
		var sessionIsNull bool
		if err = rows.Scan(&id, &status, &sessionIsNull); err != nil {
			t.Fatal(err)
		}
		if !sessionIsNull {
			t.Fatalf("pre-migration request %s unexpectedly gained a browser session", id)
		}
		got[id] = status
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["plain-pending"] != "pending" || got["already-consumed"] != "consumed" {
		t.Fatalf("migration 25 retained unsafe pre-session links: %#v", got)
	}
}
