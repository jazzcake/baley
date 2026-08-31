package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration16AddsPilotMeasurementRecordType(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "migration-16-integration-secret")
	deletePilotMeasurementRecords(t, url)
	migrations := filepath.Join("..", "migrations")
	// Step from latest (23) to the schema immediately before migration 16.
	for range 8 {
		if err := postgres.Migrate(url, migrations, "down"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore migration 16: %v", err)
		}
	})

	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `
		SET session_replication_role='replica';
		TRUNCATE task_record_indexes,repositories,runs,tasks,lanes,phases,
		  workspace_counters,workspaces,actors CASCADE;
		SET session_replication_role='origin';
		INSERT INTO workspaces(id,name) VALUES('migration-16','Migration 16');
		INSERT INTO phases(workspace_id,id,name,position,state)
		  VALUES('migration-16','pilot','Pilot',1,'active');
		INSERT INTO lanes(workspace_id,id,name,state) VALUES('migration-16','adoption','Adoption','active');
		INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,title,status)
		  VALUES('migration-16','task-1',1,'adoption','pilot','Measure','in_progress');
		INSERT INTO repositories(workspace_id,id,name,remote_url,is_record_repository,task_records_root)
		  VALUES('migration-16','6279cb62-d52f-4642-942c-15e7bd72c001','Main','https://example.test/repo.git',true,'task-records');
	`); err != nil {
		repo.Pool.Close()
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO task_record_indexes(
		workspace_id,id,task_id,record_type,repository_id,relative_path,state,short_summary
	) VALUES(
		'migration-16','6279cb62-d52f-4642-942c-15e7bd72c002','task-1','pilot-measurement',
		'6279cb62-d52f-4642-942c-15e7bd72c001','task-records/pilot.md',
		'reported_uncommitted','pilot'
	)`); err == nil {
		repo.Pool.Close()
		t.Fatal("pre-migration schema accepted pilot-measurement")
	}
	repo.Pool.Close()

	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	repo, err = postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO task_record_indexes(
		workspace_id,id,task_id,record_type,repository_id,relative_path,state,short_summary
	) VALUES(
		'migration-16','6279cb62-d52f-4642-942c-15e7bd72c002','task-1','pilot-measurement',
		'6279cb62-d52f-4642-942c-15e7bd72c001','task-records/pilot.md',
		'reported_uncommitted','pilot'
	)`); err != nil {
		t.Fatalf("post-migration schema rejected pilot-measurement: %v", err)
	}
	// Remove migrations 23 through 17. The following downgrade is migration 16
	// and must reject the live Record.
	for range 7 {
		if err = postgres.Migrate(url, migrations, "down"); err != nil {
			t.Fatalf("remove migration above 16 before downgrade check: %v", err)
		}
	}
	if err = postgres.Migrate(url, migrations, "down"); err == nil {
		t.Fatal("downgrade deleted an existing pilot-measurement Record")
	}
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM task_record_indexes WHERE workspace_id='migration-16'"); err != nil {
		t.Fatal(err)
	}
}

func deletePilotMeasurementRecords(t *testing.T, url string) {
	t.Helper()
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM task_record_indexes WHERE record_type='pilot-measurement'"); err != nil {
		t.Fatal(err)
	}
}
