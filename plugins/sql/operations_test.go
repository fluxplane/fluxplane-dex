package sql

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding/plugintest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestQueryRejectsWrites(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationQuery, map[string]any{"url": "mysql://db:3306/app", "query": "delete from users"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestTargetFromPartsBuildsRedactedMySQLDSN(t *testing.T) {
	target, err := targetFromNetworkParts("mysql", "timo", "secret", true, "db.example.com", "3307", "app", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.DSN != "timo:secret@tcp(db.example.com:3307)/app?parseTime=true" {
		t.Fatalf("dsn = %q", target.DSN)
	}
	if target.SafeURL != "mysql://timo:xxxxx@db.example.com:3307/app" {
		t.Fatalf("safe url = %q", target.SafeURL)
	}
	if target.Database != "app" {
		t.Fatalf("database = %q", target.Database)
	}
}

func TestTargetFromCredentialCanUseManualEndpoint(t *testing.T) {
	target, err := targetFromCredential("", "mysql://127.0.0.1:3306/dev", "", credentialMaterial{Username: "root"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.DSN != "root@tcp(127.0.0.1:3306)/dev?parseTime=true" {
		t.Fatalf("dsn = %q", target.DSN)
	}
	if target.SafeURL != "mysql://root@127.0.0.1:3306/dev" {
		t.Fatalf("safe url = %q", target.SafeURL)
	}
}

func TestTargetFromSecretJSON(t *testing.T) {
	target, ok, err := targetFromSecret(`{"username":"app","password":"secret","host":"mysql.local","port":3306,"database":"dev"}`, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected JSON secret to be recognized")
	}
	if target.DSN != "app:secret@tcp(mysql.local:3306)/dev?parseTime=true" {
		t.Fatalf("dsn = %q", target.DSN)
	}
	if target.SafeURL != "mysql://app:xxxxx@mysql.local:3306/dev" {
		t.Fatalf("safe url = %q", target.SafeURL)
	}
}

func TestTargetFromPostgresURL(t *testing.T) {
	target, err := targetFromURL("", "postgres://app:secret@db.example.com:5433/app?sslmode=disable", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if target.Driver != "pgx" || target.Dialect != "postgres" {
		t.Fatalf("driver = %q", target.Driver)
	}
	if target.DSN != "postgres://app:secret@db.example.com:5433/app?sslmode=disable" {
		t.Fatalf("dsn = %q", target.DSN)
	}
	if target.SafeURL != "postgres://app:xxxxx@db.example.com:5433/app?sslmode=disable" {
		t.Fatalf("safe url = %q", target.SafeURL)
	}
}

func TestQueryRunsAgainstSQLite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "app.db")
	db, err := stdsql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table users (id integer primary key, name text); insert into users (id, name) values (1, 'Ada'), (2, 'Linus')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[QueryOutput](t, plugin, OperationQuery, map[string]any{
		"driver":   "sqlite",
		"dsn":      dbPath,
		"query":    "select id, name from users order by id",
		"max_rows": 10,
	})
	if out.Driver != "sqlite" || out.RowCount != 2 || out.Rows[0]["name"] != "Ada" || out.Rows[1]["name"] != "Linus" {
		t.Fatalf("out = %#v", out)
	}

	records := plugintest.DatasourceSearchOK[QueryRowsResult](t, plugin, map[string]any{
		"datasource": DatasourceQueryRows,
		"driver":     "sqlite",
		"dsn":        dbPath,
		"query":      "select id, name from users order by id",
		"max_rows":   10,
	})
	if records.Count != 2 || records.Records[0].Row["name"] != "Ada" || records.Records[0].Driver != "sqlite" {
		t.Fatalf("records = %#v", records)
	}
}

func TestQueryRunsAgainstMySQLContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container-backed integration test in short mode")
	}
	ctx := context.Background()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mysql:8.4",
			ExposedPorts: []string{"3306/tcp"},
			Env: map[string]string{
				"MYSQL_DATABASE":      "app",
				"MYSQL_USER":          "app",
				"MYSQL_PASSWORD":      "secret",
				"MYSQL_ROOT_PASSWORD": "root-secret",
			},
			WaitingFor: wait.ForListeningPort("3306/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Fatalf("terminate container: %v", err)
		}
	}()

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "3306/tcp")
	if err != nil {
		t.Fatal(err)
	}
	dsn := fmt.Sprintf("app:secret@tcp(%s)/app?parseTime=true", net.JoinHostPort(host, port.Port()))
	db, err := stdsql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `create table users (id int primary key, name varchar(64))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `insert into users (id, name) values (1, 'Ada'), (2, 'Linus')`); err != nil {
		t.Fatal(err)
	}

	get := func(_ pluginbinding.Context, purpose string) (pluginbinding.SecretMaterial, error) {
		switch purpose {
		case AuthPurposeUsername:
			return pluginbinding.SecretMaterial{Purpose: purpose, Value: "app"}, nil
		case AuthPurposePassword:
			return pluginbinding.SecretMaterial{Purpose: purpose, Value: "secret"}, nil
		default:
			return pluginbinding.SecretMaterial{}, nil
		}
	}
	plugin := NewPluginWithService(Service{SecretGetter: get})
	out := plugintest.RunOK[QueryOutput](t, plugin, OperationQuery, map[string]any{
		"url":      "mysql://" + net.JoinHostPort(host, port.Port()) + "/app",
		"query":    "select id, name from users order by id",
		"max_rows": 10,
	})
	if out.RowCount != 2 || out.Rows[0]["name"] != "Ada" || out.Rows[1]["name"] != "Linus" {
		t.Fatalf("out = %#v", out)
	}
	if out.EndpointURL != "mysql://app:xxxxx@"+net.JoinHostPort(host, port.Port())+"/app" {
		t.Fatalf("endpoint url = %q", out.EndpointURL)
	}
}

func TestQueryRunsAgainstPostgresContainer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping container-backed integration test in short mode")
	}
	ctx := context.Background()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_DB":       "app",
				"POSTGRES_USER":     "app",
				"POSTGRES_PASSWORD": "secret",
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Fatalf("terminate container: %v", err)
		}
	}()

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	dsn := fmt.Sprintf("postgres://app:secret@%s/app?sslmode=disable", net.JoinHostPort(host, port.Port()))
	db, err := stdsql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `create table users (id integer primary key, name text)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `insert into users (id, name) values (1, 'Ada'), (2, 'Linus')`); err != nil {
		t.Fatal(err)
	}

	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[QueryOutput](t, plugin, OperationQuery, map[string]any{
		"url":      "postgres://app:secret@" + net.JoinHostPort(host, port.Port()) + "/app?sslmode=disable",
		"query":    "select id, name from users order by id",
		"max_rows": 10,
	})
	if out.Driver != "postgres" || out.RowCount != 2 || out.Rows[0]["name"] != "Ada" || out.Rows[1]["name"] != "Linus" {
		t.Fatalf("out = %#v", out)
	}
}
