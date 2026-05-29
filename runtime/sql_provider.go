package runtime

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core/pluginbinding"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type sqlProviderQueryInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty"`
	Driver      string `json:"driver,omitempty"`
	Database    string `json:"database,omitempty"`
	Query       string `json:"query,omitempty"`
	Timeout     string `json:"timeout,omitempty"`
	MaxRows     int    `json:"max_rows,omitempty"`
}

type sqlProviderQueryOutput struct {
	EndpointRef string           `json:"endpoint_ref,omitempty"`
	EndpointURL string           `json:"endpoint_url,omitempty"`
	Driver      string           `json:"driver,omitempty"`
	Database    string           `json:"database,omitempty"`
	Columns     []string         `json:"columns,omitempty"`
	Rows        []map[string]any `json:"rows,omitempty"`
	RowCount    int              `json:"row_count"`
	Truncated   bool             `json:"truncated,omitempty"`
	DurationMS  int64            `json:"duration_ms,omitempty"`
}

type sqlProviderTarget struct {
	Driver   string
	Dialect  string
	DSN      string
	SafeURL  string
	Database string
}

func (r Runner) hostSQLProviderCall(ctx context.Context, plugin, instance, grant string, input pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	if strings.TrimSpace(input.Action) != "query" {
		return pluginbinding.ProviderCallResponse{}, fmt.Errorf("unsupported SQL provider action %q", input.Action)
	}
	var queryInput sqlProviderQueryInput
	if len(input.Payload) > 0 {
		if err := json.Unmarshal(input.Payload, &queryInput); err != nil {
			return pluginbinding.ProviderCallResponse{}, err
		}
	}
	out, err := r.runHostSQLQuery(ctx, plugin, instance, grant, queryInput)
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return pluginbinding.ProviderCallResponse{}, err
	}
	return pluginbinding.ProviderCallResponse{Result: raw}, nil
}

func (r Runner) runHostSQLQuery(ctx context.Context, plugin, instance, grant string, input sqlProviderQueryInput) (sqlProviderQueryOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return sqlProviderQueryOutput{}, fmt.Errorf("query is required")
	}
	timeout, err := sqlProviderDuration(input.Timeout, 10*time.Second)
	if err != nil {
		return sqlProviderQueryOutput{}, err
	}
	maxRows := input.MaxRows
	if maxRows <= 0 {
		maxRows = 100
	}
	if maxRows > 1000 {
		maxRows = 1000
	}
	target, err := r.sqlProviderTarget(ctx, plugin, instance, grant, input)
	if err != nil {
		return sqlProviderQueryOutput{}, err
	}
	db, err := stdsql.Open(target.Driver, target.DSN)
	if err != nil {
		return sqlProviderQueryOutput{}, err
	}
	defer func() { _ = db.Close() }()
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return sqlProviderQueryOutput{}, err
	}
	defer func() { _ = rows.Close() }()
	resultRows, columns, truncated, err := sqlProviderScanRows(rows, maxRows)
	if err != nil {
		return sqlProviderQueryOutput{}, err
	}
	return sqlProviderQueryOutput{
		EndpointRef: input.EndpointRef,
		EndpointURL: target.SafeURL,
		Driver:      firstNonEmpty(target.Dialect, target.Driver),
		Database:    target.Database,
		Columns:     columns,
		Rows:        resultRows,
		RowCount:    len(resultRows),
		Truncated:   truncated,
		DurationMS:  time.Since(start).Milliseconds(),
	}, nil
}

func (r Runner) sqlProviderTarget(ctx context.Context, plugin, instance, grant string, input sqlProviderQueryInput) (sqlProviderTarget, error) {
	endpointRef := strings.TrimSpace(input.EndpointRef)
	if endpointRef == "" {
		return sqlProviderTarget{}, fmt.Errorf("endpoint_ref is required")
	}
	endpoint, ok, err := r.State.GetEndpoint(endpointRef)
	if err != nil {
		return sqlProviderTarget{}, err
	}
	if !ok {
		return sqlProviderTarget{}, fmt.Errorf("unknown endpoint_ref %q", endpointRef)
	}
	username := r.optionalHostSecret(ctx, plugin, instance, "username", grant)
	password := r.optionalHostSecret(ctx, plugin, instance, "password", grant)
	if target, ok, err := sqlProviderTargetFromSecret(password, input.Driver, input.Database); ok || err != nil {
		return target, err
	}
	return sqlProviderTargetFromURL(input.Driver, endpoint.URL, input.Database, username, password)
}

func (r Runner) optionalHostSecret(ctx context.Context, plugin, instance, purpose, grant string) string {
	material, err := r.State.ResolveSecret(ctx, plugin, instance, purpose, grant)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(material.Value)
}

func sqlProviderDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func sqlProviderTargetFromSecret(secretValue, driverOverride, databaseOverride string) (sqlProviderTarget, bool, error) {
	secretValue = strings.TrimSpace(secretValue)
	if secretValue == "" {
		return sqlProviderTarget{}, false, nil
	}
	if strings.Contains(secretValue, "://") {
		target, err := sqlProviderTargetFromURL(driverOverride, secretValue, databaseOverride, "", "")
		return target, true, err
	}
	if strings.HasPrefix(secretValue, "{") {
		var payload struct {
			Username string `json:"username"`
			User     string `json:"user"`
			Password string `json:"password"`
			Driver   string `json:"driver"`
			URL      string `json:"url"`
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Database string `json:"database"`
			DB       string `json:"db"`
			DSN      string `json:"dsn"`
		}
		if err := json.Unmarshal([]byte(secretValue), &payload); err != nil {
			return sqlProviderTarget{}, true, err
		}
		driver := firstNonEmpty(driverOverride, payload.Driver)
		database := firstNonEmpty(databaseOverride, payload.Database, payload.DB)
		if payload.URL != "" {
			target, err := sqlProviderTargetFromURL(driver, payload.URL, database, firstNonEmpty(payload.Username, payload.User), payload.Password)
			return target, true, err
		}
		if payload.DSN != "" {
			target, err := sqlProviderTargetFromDSN(driver, payload.DSN, database)
			return target, true, err
		}
		port := ""
		if payload.Port > 0 {
			port = strconv.Itoa(payload.Port)
		}
		if payload.Host == "" {
			return sqlProviderTarget{}, true, fmt.Errorf("SQL secret JSON must include url, dsn, or host")
		}
		if driver == "" {
			driver = "mysql"
		}
		target, err := sqlProviderTargetFromNetworkParts(driver, firstNonEmpty(payload.Username, payload.User, sqlProviderDefaultUser(driver)), payload.Password, payload.Password != "", payload.Host, port, database, "")
		return target, true, err
	}
	if sqlProviderLooksLikeMySQLDriverDSN(secretValue) {
		database := sqlProviderDatabaseFromDriverDSN(secretValue)
		if strings.TrimSpace(databaseOverride) != "" {
			database = strings.TrimSpace(databaseOverride)
		}
		return sqlProviderTarget{Driver: "mysql", Dialect: "mysql", DSN: secretValue, SafeURL: sqlProviderRedactDSN(secretValue), Database: database}, true, nil
	}
	if driver := sqlProviderNormalizeDriver(driverOverride, ""); driver != "" && driver != "mysql" {
		target, err := sqlProviderTargetFromDSN(driver, secretValue, databaseOverride)
		return target, true, err
	}
	return sqlProviderTarget{}, false, nil
}

func sqlProviderTargetFromURL(driverOverride, rawURL, databaseOverride, usernameOverride, passwordOverride string) (sqlProviderTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return sqlProviderTarget{}, err
	}
	driver := sqlProviderNormalizeDriver(driverOverride, parsed.Scheme)
	if driver == "" {
		return sqlProviderTarget{}, fmt.Errorf("unsupported SQL URL scheme %q", parsed.Scheme)
	}
	if !sqlProviderSupportedDriver(driver) {
		return sqlProviderTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	if driver == "sqlite" {
		return sqlProviderTargetFromSQLiteURL(parsed, rawURL, databaseOverride), nil
	}
	host := parsed.Hostname()
	if host == "" {
		return sqlProviderTarget{}, fmt.Errorf("endpoint URL has no host")
	}
	port := parsed.Port()
	user := parsed.User.Username()
	pass, hasPass := parsed.User.Password()
	if strings.TrimSpace(usernameOverride) != "" {
		user = usernameOverride
	}
	if strings.TrimSpace(passwordOverride) != "" {
		pass = passwordOverride
		hasPass = true
	}
	if user == "" {
		user = sqlProviderDefaultUser(driver)
	}
	database := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	return sqlProviderTargetFromNetworkParts(driver, user, pass, hasPass, host, port, database, parsed.RawQuery)
}

func sqlProviderTargetFromDSN(driverOverride, dsn, databaseOverride string) (sqlProviderTarget, error) {
	dsn = strings.TrimSpace(dsn)
	driver := sqlProviderNormalizeDriver(driverOverride, "")
	if driver == "" {
		driver = sqlProviderDetectDSNDriver(dsn)
	}
	if driver == "" {
		return sqlProviderTarget{}, fmt.Errorf("driver is required when using a raw DSN")
	}
	if !sqlProviderSupportedDriver(driver) {
		return sqlProviderTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	if driver == "mysql" && !sqlProviderLooksLikeMySQLDriverDSN(dsn) && strings.Contains(dsn, "://") {
		return sqlProviderTargetFromURL(driver, dsn, databaseOverride, "", "")
	}
	if driver == "pgx" && strings.Contains(dsn, "://") {
		return sqlProviderTargetFromURL(driver, dsn, databaseOverride, "", "")
	}
	database := sqlProviderDatabaseFromDriverDSN(dsn)
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	return sqlProviderTarget{Driver: driver, Dialect: sqlProviderDialectName(driver), DSN: dsn, SafeURL: sqlProviderSafeDSN(driver, dsn), Database: database}, nil
}

func sqlProviderTargetFromNetworkParts(driver, user, password string, hasPassword bool, host, port, database, query string) (sqlProviderTarget, error) {
	driver = sqlProviderNormalizeDriver(driver, "")
	if driver == "pgx" {
		return sqlProviderPostgresTargetFromParts(user, password, hasPassword, host, port, database, query), nil
	}
	if driver != "mysql" {
		return sqlProviderTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	if port == "" {
		port = "3306"
	}
	addr := net.JoinHostPort(host, port)
	auth := user
	if hasPassword {
		auth += ":" + password
	}
	dsn := auth + "@tcp(" + addr + ")/" + database + "?parseTime=true"
	safeAuth := user
	if hasPassword {
		safeAuth += ":xxxxx"
	}
	safeURL := "mysql://" + safeAuth + "@" + addr
	if database != "" {
		safeURL += "/" + database
	}
	return sqlProviderTarget{Driver: "mysql", Dialect: "mysql", DSN: dsn, SafeURL: safeURL, Database: database}, nil
}

func sqlProviderPostgresTargetFromParts(user, password string, hasPassword bool, host, port, database, query string) sqlProviderTarget {
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	parsed := url.URL{Scheme: "postgres", Host: host, RawQuery: query}
	if user != "" && hasPassword {
		parsed.User = url.UserPassword(user, password)
	} else if user != "" {
		parsed.User = url.User(user)
	}
	if database != "" {
		parsed.Path = "/" + database
	}
	dsn := parsed.String()
	safe := parsed
	if safe.User != nil {
		if safe.User.Username() != "" && hasPassword {
			safe.User = url.UserPassword(safe.User.Username(), "xxxxx")
		}
	}
	return sqlProviderTarget{Driver: "pgx", Dialect: "postgres", DSN: dsn, SafeURL: safe.String(), Database: database}
}

func sqlProviderTargetFromSQLiteURL(parsed *url.URL, rawURL, databaseOverride string) sqlProviderTarget {
	dsn := strings.TrimSpace(rawURL)
	switch parsed.Scheme {
	case "sqlite", "sqlite3":
		switch {
		case parsed.Opaque != "":
			dsn = parsed.Opaque
		case parsed.Host == "" && parsed.Path != "":
			dsn = parsed.Path
		case parsed.Host != "" && parsed.Path != "":
			dsn = strings.TrimLeft(parsed.Host+parsed.Path, "/")
		case parsed.Host != "":
			dsn = parsed.Host
		}
	case "file":
		dsn = rawURL
	}
	database := dsn
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	return sqlProviderTarget{Driver: "sqlite", Dialect: "sqlite", DSN: dsn, SafeURL: "sqlite://" + dsn, Database: database}
}

func sqlProviderNormalizeDriver(driver, scheme string) string {
	value := strings.ToLower(strings.TrimSpace(firstNonEmpty(driver, scheme)))
	switch value {
	case "mysql", "mariadb":
		return "mysql"
	case "postgres", "postgresql", "pg", "pgx":
		return "pgx"
	case "sqlite", "sqlite3", "file":
		return "sqlite"
	default:
		return value
	}
}

func sqlProviderDefaultUser(driver string) string {
	switch sqlProviderNormalizeDriver(driver, "") {
	case "mysql":
		return "root"
	case "pgx":
		return "postgres"
	default:
		return ""
	}
}

func sqlProviderSupportedDriver(driver string) bool {
	switch sqlProviderNormalizeDriver(driver, "") {
	case "mysql", "pgx", "sqlite":
		return true
	default:
		return false
	}
}

func sqlProviderDialectName(driver string) string {
	switch sqlProviderNormalizeDriver(driver, "") {
	case "pgx":
		return "postgres"
	default:
		return sqlProviderNormalizeDriver(driver, "")
	}
}

func sqlProviderDetectDSNDriver(dsn string) string {
	switch {
	case sqlProviderLooksLikeMySQLDriverDSN(dsn):
		return "mysql"
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"), strings.Contains(dsn, "host="):
		return "pgx"
	default:
		return ""
	}
}

func sqlProviderSafeDSN(driver, dsn string) string {
	switch sqlProviderNormalizeDriver(driver, "") {
	case "mysql", "pgx":
		return sqlProviderRedactDSN(dsn)
	case "sqlite":
		return "sqlite://" + dsn
	default:
		return dsn
	}
}

func sqlProviderScanRows(rows *stdsql.Rows, maxRows int) ([]map[string]any, []string, bool, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	var out []map[string]any
	truncated := false
	for rows.Next() {
		if len(out) >= maxRows {
			truncated = true
			break
		}
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, false, err
		}
		row := map[string]any{}
		for i, column := range columns {
			value := values[i]
			if bytes, ok := value.([]byte); ok {
				value = string(bytes)
			}
			row[column] = value
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return out, columns, truncated, nil
}

func sqlProviderLooksLikeMySQLDriverDSN(value string) bool {
	return strings.Contains(value, "@tcp(") || strings.Contains(value, "@unix(")
}

func sqlProviderDatabaseFromDriverDSN(dsn string) string {
	if parsed, err := url.Parse(dsn); err == nil && parsed.Scheme != "" {
		return strings.Trim(strings.TrimSpace(parsed.Path), "/")
	}
	for _, field := range strings.Fields(dsn) {
		key, value, ok := strings.Cut(field, "=")
		if ok && key == "dbname" {
			return value
		}
	}
	if idx := strings.LastIndex(dsn, ")/"); idx >= 0 {
		rest := dsn[idx+2:]
		if q := strings.Index(rest, "?"); q >= 0 {
			rest = rest[:q]
		}
		return rest
	}
	return ""
}

func sqlProviderRedactDSN(dsn string) string {
	if at := strings.Index(dsn, "@"); at > 0 {
		userInfo := dsn[:at]
		if colon := strings.Index(userInfo, ":"); colon >= 0 {
			return userInfo[:colon] + ":xxxxx" + dsn[at:]
		}
	}
	return dsn
}
