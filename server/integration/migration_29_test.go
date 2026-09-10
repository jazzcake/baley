package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration29MakesEventsAppendOnlyAndRollsBack(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	migrations := filepath.Join("..", "migrations")
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if err = postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM events WHERE id IN ('migration29-event','migration29-insert'); DELETE FROM commands WHERE id='migration29-command'"); err != nil {
		t.Fatal(err)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO commands(id,workspace_id,idempotency_key,command_name,command_hash,request_fingerprint,workspace_revision,result,executed_by_actor_id)
		VALUES('migration29-command',$1,'migration29-command','migration29.test','hash','fingerprint',1,'{}'::jsonb,$2) ON CONFLICT (id) DO NOTHING`, postgres.DemoWorkspaceID, postgres.DemoAgentActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO events(id,workspace_id,command_id,workspace_revision,command_event_index,event_type,entity_type,entity_id,executed_by_actor_id,payload)
		VALUES('migration29-event',$1,'migration29-command',1,0,'migration29.original','event','migration29',$2,'{"binding":"A"}'::jsonb) ON CONFLICT (id) DO NOTHING`, postgres.DemoWorkspaceID, postgres.DemoAgentActorID); err != nil {
		t.Fatal(err)
	}
	var eventID, originalPayload string
	if err = repo.Pool.QueryRow(ctx, "SELECT id,payload::text FROM events WHERE id='migration29-event'").Scan(&eventID, &originalPayload); err != nil {
		t.Fatal(err)
	}
	for name, statement := range map[string]string{
		"update":   "UPDATE events SET payload='{}'::jsonb WHERE id=$1",
		"delete":   "DELETE FROM events WHERE id=$1",
		"truncate": "TRUNCATE events CASCADE",
	} {
		t.Run(name, func(t *testing.T) {
			var mutationErr error
			if name == "truncate" {
				_, mutationErr = repo.Pool.Exec(ctx, statement)
			} else {
				_, mutationErr = repo.Pool.Exec(ctx, statement, eventID)
			}
			if mutationErr == nil || !strings.Contains(mutationErr.Error(), "events is append-only") {
				t.Fatalf("%s did not fail at append-only boundary: %v", name, mutationErr)
			}
			var payload string
			if scanErr := repo.Pool.QueryRow(ctx, "SELECT payload::text FROM events WHERE id=$1", eventID).Scan(&payload); scanErr != nil || payload != originalPayload {
				t.Fatalf("original event changed after %s: payload=%s err=%v", name, payload, scanErr)
			}
		})
	}
	var before int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO events(id,workspace_id,command_id,workspace_revision,command_event_index,event_type,entity_type,entity_id,executed_by_actor_id,payload) SELECT 'migration29-insert',workspace_id,command_id,workspace_revision,command_event_index+100,'migration29.test','event','migration29',executed_by_actor_id,'{}'::jsonb FROM events WHERE id=$1`, eventID); err != nil {
		t.Fatalf("normal insert failed: %v", err)
	}
	var after int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM events").Scan(&after); err != nil || after != before+1 {
		t.Fatalf("insert count=%d want=%d err=%v", after, before+1, err)
	}
	if err = postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM events WHERE id='migration29-insert'"); err != nil {
		t.Fatalf("down migration did not restore mutation capability: %v", err)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
}
