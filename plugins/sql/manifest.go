package sql

import (
	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
)

const (
	PluginName        = "sql"
	PluginVersion     = "0.1.0"
	PluginDescription = "Read-only SQL query operations for MySQL, PostgreSQL, SQLite, and compatible endpoints."

	AuthMethodSQL        = "sql"
	AuthPurposeUsername  = "username"
	AuthPurposePassword  = "password"
	EnvSQLUsername       = "SQL_USERNAME"
	EnvSQLPassword       = "SQL_PASSWORD"
	EnvMySQLUsername     = "MYSQL_USERNAME"
	EnvMySQLPassword     = "MYSQL_PASSWORD"
	OperationQuery       = "sql.query"
	DatasourceQueryRows  = "sql.query_rows"
	EntitySQLQueryResult = "sql.query_result"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"mysql", PluginName},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodSQL,
			Kind:        "credentials",
			Description: "SQL credentials resolved by dex secret broker.",
			Env:         []string{EnvSQLUsername, EnvSQLPassword, EnvMySQLUsername, EnvMySQLPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeUsername, "SQL username", false, true, EnvSQLUsername, EnvMySQLUsername),
				pluginbinding.AuthField(AuthPurposePassword, "SQL password, URL, or DSN", false, true, EnvSQLPassword, EnvMySQLPassword),
			},
		}},
		Operations: []core.OperationSpec{querySpec()},
	}
}

func querySpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[QueryInput, QueryOutput](
		OperationQuery,
		"Run a bounded read-only SQL query against a SQL endpoint.",
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeUsername, AuthPurposePassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}
