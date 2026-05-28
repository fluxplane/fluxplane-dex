package sql

import (
	"context"
	"crypto/sha1"
	stdsql "database/sql"
	"encoding/hex"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	_ "modernc.org/sqlite"
)

type Queryer interface {
	QueryContext(context.Context, string, ...any) (*stdsql.Rows, error)
	Close() error
}

type Opener func(driverName, dsn string) (Queryer, error)

type Service struct {
	Open         Opener
	SecretGetter pluginbinding.SecretGetter
}

func NewService() Service {
	return Service{Open: defaultOpen}
}

type QueryInput struct {
	EndpointRef   string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered endpoint ref resolved by the host."`
	Driver        string `json:"driver,omitempty" jsonschema:"description=SQL driver or dialect.,enum=mysql,enum=postgres,enum=sqlite"`
	URL           string `json:"url,omitempty" jsonschema:"description=SQL endpoint URL."`
	DSN           string `json:"dsn,omitempty" jsonschema:"description=Driver DSN, mainly for local SQLite or advanced database connections."`
	CredentialRef string `json:"credential_ref,omitempty" jsonschema:"description=Credential reference associated with the endpoint."`
	Database      string `json:"database,omitempty" jsonschema:"description=Database override."`
	Query         string `json:"query,omitempty" jsonschema:"required,description=Read-only SQL query."`
	Timeout       string `json:"timeout,omitempty" jsonschema:"description=Query timeout duration."`
	MaxRows       int    `json:"max_rows,omitempty" jsonschema:"description=Maximum rows to return."`
}

type QueryOutput struct {
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

type QueryRowRecord struct {
	pluginbinding.DatasourceRecord
	RowID       string         `json:"row_id" datasource:"id,completion,view=compact|lookup|table"`
	Title       string         `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Columns     []string       `json:"columns,omitempty" datasource:"view=table"`
	Row         map[string]any `json:"row,omitempty" datasource:"view=compact|lookup|table"`
	Driver      string         `json:"driver,omitempty" datasource:"completion,view=compact|lookup|table"`
	Database    string         `json:"database,omitempty" datasource:"completion,view=compact|lookup|table"`
	EndpointURL string         `json:"endpoint_url,omitempty" datasource:"completion,view=lookup|table"`
}

type QueryRowsResult = pluginbinding.DatasourceSearchResult[QueryRowRecord]

const readOnlySQLQueryMessage = "SQL query must be read-only; allowed statements are SELECT, SHOW, DESCRIBE, EXPLAIN, and WITH"

func (s Service) Query(ctx pluginbinding.Context, input QueryInput) (QueryOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return QueryOutput{}, pluginbinding.Fail("bad_input", "query is required")
	}
	if !readOnlyQuery(query) {
		return QueryOutput{}, pluginbinding.Fail("bad_input", readOnlySQLQueryMessage)
	}
	timeout, err := parseDurationDefault(input.Timeout, 10*time.Second)
	if err != nil {
		return QueryOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	maxRows := input.MaxRows
	if maxRows <= 0 {
		maxRows = 100
	}
	if maxRows > 1000 {
		maxRows = 1000
	}
	target, err := s.target(ctx, input)
	if err != nil {
		return QueryOutput{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	open := s.Open
	if open == nil {
		open = defaultOpen
	}
	db, err := open(target.Driver, target.DSN)
	if err != nil {
		return QueryOutput{}, pluginbinding.Errorf("sql", "%s", err)
	}
	defer db.Close()
	queryCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	rows, err := db.QueryContext(queryCtx, query)
	if err != nil {
		return QueryOutput{}, pluginbinding.Errorf("sql", "%s", err)
	}
	defer rows.Close()
	resultRows, columns, truncated, err := scanRows(rows, maxRows)
	if err != nil {
		return QueryOutput{}, pluginbinding.Errorf("sql", "%s", err)
	}
	return QueryOutput{
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

func (s Service) QueryRows(ctx pluginbinding.Context, input QueryInput) (QueryRowsResult, error) {
	if strings.TrimSpace(input.Query) != "" && !readOnlyQuery(input.Query) {
		return QueryRowsResult{}, pluginbinding.Fail("bad_input", "SQL datasource search requires a read-only SQL query; pass SELECT, SHOW, DESCRIBE, EXPLAIN, or WITH SQL with --query")
	}
	out, err := s.Query(ctx, input)
	if err != nil {
		return QueryRowsResult{}, err
	}
	records := make([]QueryRowRecord, 0, len(out.Rows))
	for i, row := range out.Rows {
		rowID := sqlRowID(out, input.Query, i, row)
		title := sqlRowTitle(i, row, out.Columns)
		metadata := map[string]any{
			"columns":      out.Columns,
			"row":          row,
			"driver":       out.Driver,
			"database":     out.Database,
			"endpoint_url": out.EndpointURL,
			"endpoint_ref": out.EndpointRef,
			"query":        input.Query,
		}
		record := QueryRowRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(
				ctx.DatasourceSource(),
				EntitySQLQueryResult,
				rowID,
				pluginbinding.RecordTitle(title),
				pluginbinding.RecordMetadata(metadata),
			),
			RowID:       rowID,
			Title:       title,
			Columns:     append([]string(nil), out.Columns...),
			Row:         row,
			Driver:      out.Driver,
			Database:    out.Database,
			EndpointURL: out.EndpointURL,
		}
		if out.EndpointURL != "" {
			record.Links = map[string]string{"endpoint": out.EndpointURL}
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.Query, records), nil
}

type sqlTarget struct {
	Driver   string
	Dialect  string
	DSN      string
	SafeURL  string
	Database string
}

func (s Service) target(ctx pluginbinding.Context, input QueryInput) (sqlTarget, error) {
	password, _ := ctx.OptionalSecret(AuthPurposePassword)
	username, _ := ctx.OptionalSecret(AuthPurposeUsername)
	if strings.TrimSpace(input.DSN) != "" {
		return targetFromDSN(input.Driver, input.DSN, input.Database)
	}
	if strings.TrimSpace(input.CredentialRef) != "" {
		credential, err := resolveCredentialRef(context.Background(), input.CredentialRef)
		if err != nil {
			return sqlTarget{}, err
		}
		return targetFromCredential(input.Driver, input.URL, input.Database, credential, username.Value, password.Value)
	}
	if target, ok, err := targetFromSecret(password.Value, input.Driver, input.Database); ok || err != nil {
		return target, err
	}
	rawURL := strings.TrimSpace(input.URL)
	if rawURL == "" {
		return sqlTarget{}, fmt.Errorf("url, dsn, or endpoint_ref is required")
	}
	return targetFromURL(input.Driver, rawURL, input.Database, username.Value, password.Value)
}

type credentialMaterial struct {
	Username string
	Password string
	Host     string
	Port     string
	Database string
}

func targetFromCredential(driverOverride, rawURL, databaseOverride string, credential credentialMaterial, usernameOverride, passwordOverride string) (sqlTarget, error) {
	parsed, err := parseOptionalURL(rawURL)
	if err != nil {
		return sqlTarget{}, err
	}
	driver := normalizeDriver(driverOverride, "")
	if parsed != nil {
		driver = normalizeDriver(driverOverride, parsed.Scheme)
	}
	if driver == "sqlite" {
		return targetFromURL(driver, rawURL, databaseOverride, "", "")
	}
	if driver == "" {
		driver = "mysql"
	}
	if !supportedDriver(driver) {
		return sqlTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	host := credential.Host
	port := credential.Port
	database := credential.Database
	query := ""
	userFromURL := ""
	passwordFromURL := ""
	hasPasswordFromURL := false
	if parsed != nil {
		host = firstNonEmpty(host, parsed.Hostname())
		port = firstNonEmpty(port, parsed.Port())
		if database == "" {
			database = strings.Trim(strings.TrimSpace(parsed.Path), "/")
		}
		query = parsed.RawQuery
		userFromURL = parsed.User.Username()
		passwordFromURL, hasPasswordFromURL = parsed.User.Password()
	}
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	user := firstNonEmpty(usernameOverride, credential.Username, userFromURL, defaultUser(driver))
	password := firstNonEmpty(passwordOverride, credential.Password, passwordFromURL)
	hasPassword := password != "" || hasPasswordFromURL
	if host == "" {
		return sqlTarget{}, fmt.Errorf("credential ref did not resolve a host")
	}
	return targetFromNetworkParts(driver, user, password, hasPassword, host, port, database, query)
}

func resolveCredentialRef(ctx context.Context, ref string) (credentialMaterial, error) {
	parsed, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return credentialMaterial{}, err
	}
	if parsed.Scheme != "kubernetes" {
		return credentialMaterial{}, fmt.Errorf("unsupported credential_ref scheme %q", parsed.Scheme)
	}
	namespace := parsed.Host
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if namespace == "" || len(parts) != 2 || parts[0] != "secrets" || parts[1] == "" {
		return credentialMaterial{}, fmt.Errorf("invalid kubernetes credential_ref")
	}
	secretName := parts[1]
	overrides := &clientcmd.ConfigOverrides{}
	if contextName := strings.TrimSpace(parsed.Query().Get("context")); contextName != "" {
		overrides.CurrentContext = contextName
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), overrides).ClientConfig()
	if err != nil {
		return credentialMaterial{}, err
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return credentialMaterial{}, err
	}
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return credentialMaterial{}, err
	}
	return credentialMaterial{
		Username: secretString(secret.Data, "username", "user"),
		Password: secretString(secret.Data, "password", "pass"),
		Host:     secretString(secret.Data, "host", "hostname", "endpoint", "address"),
		Port:     secretString(secret.Data, "port"),
		Database: secretString(secret.Data, "database", "dbname", "db"),
	}, nil
}

func secretString(data map[string][]byte, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(string(data[key])); value != "" {
			return value
		}
	}
	return ""
}

func targetFromSecret(secretValue, driverOverride, databaseOverride string) (sqlTarget, bool, error) {
	secretValue = strings.TrimSpace(secretValue)
	if secretValue == "" {
		return sqlTarget{}, false, nil
	}
	if strings.Contains(secretValue, "://") {
		target, err := targetFromURL(driverOverride, secretValue, databaseOverride, "", "")
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
			return sqlTarget{}, true, err
		}
		driver := firstNonEmpty(driverOverride, payload.Driver)
		database := firstNonEmpty(databaseOverride, payload.Database, payload.DB)
		if payload.URL != "" {
			target, err := targetFromURL(driver, payload.URL, database, firstNonEmpty(payload.Username, payload.User), payload.Password)
			return target, true, err
		}
		if payload.DSN != "" {
			target, err := targetFromDSN(driver, payload.DSN, database)
			return target, true, err
		}
		port := ""
		if payload.Port > 0 {
			port = strconv.Itoa(payload.Port)
		}
		if payload.Host == "" {
			return sqlTarget{}, true, fmt.Errorf("SQL secret JSON must include url, dsn, or host")
		}
		if driver == "" {
			driver = "mysql"
		}
		target, err := targetFromNetworkParts(driver, firstNonEmpty(payload.Username, payload.User, defaultUser(driver)), payload.Password, payload.Password != "", payload.Host, port, database, "")
		return target, true, err
	}
	if looksLikeMySQLDriverDSN(secretValue) {
		database := databaseFromDriverDSN(secretValue)
		if strings.TrimSpace(databaseOverride) != "" {
			database = strings.TrimSpace(databaseOverride)
		}
		return sqlTarget{Driver: "mysql", Dialect: "mysql", DSN: secretValue, SafeURL: redactDSN(secretValue), Database: database}, true, nil
	}
	if driver := normalizeDriver(driverOverride, ""); driver != "" && driver != "mysql" {
		target, err := targetFromDSN(driver, secretValue, databaseOverride)
		return target, true, err
	}
	return sqlTarget{}, false, nil
}

func targetFromURL(driverOverride, rawURL, databaseOverride, usernameOverride, passwordOverride string) (sqlTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return sqlTarget{}, err
	}
	driver := normalizeDriver(driverOverride, parsed.Scheme)
	if driver == "" {
		return sqlTarget{}, fmt.Errorf("unsupported SQL URL scheme %q", parsed.Scheme)
	}
	if !supportedDriver(driver) {
		return sqlTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	if driver == "sqlite" {
		return targetFromSQLiteURL(parsed, rawURL, databaseOverride), nil
	}
	host := parsed.Hostname()
	if host == "" {
		return sqlTarget{}, fmt.Errorf("endpoint URL has no host")
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
		user = defaultUser(driver)
	}
	database := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	return targetFromNetworkParts(driver, user, pass, hasPass, host, port, database, parsed.RawQuery)
}

func targetFromDSN(driverOverride, dsn, databaseOverride string) (sqlTarget, error) {
	dsn = strings.TrimSpace(dsn)
	driver := normalizeDriver(driverOverride, "")
	if driver == "" {
		driver = detectDSNDriver(dsn)
	}
	if driver == "" {
		return sqlTarget{}, fmt.Errorf("driver is required when using a raw DSN")
	}
	if !supportedDriver(driver) {
		return sqlTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	if driver == "mysql" && !looksLikeMySQLDriverDSN(dsn) && strings.Contains(dsn, "://") {
		return targetFromURL(driver, dsn, databaseOverride, "", "")
	}
	if driver == "pgx" && strings.Contains(dsn, "://") {
		return targetFromURL(driver, dsn, databaseOverride, "", "")
	}
	database := databaseFromDriverDSN(dsn)
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	return sqlTarget{Driver: driver, Dialect: dialectName(driver), DSN: dsn, SafeURL: safeDSN(driver, dsn), Database: database}, nil
}

func targetFromNetworkParts(driver, user, password string, hasPassword bool, host, port, database, query string) (sqlTarget, error) {
	driver = normalizeDriver(driver, "")
	if driver == "pgx" {
		return postgresTargetFromParts(user, password, hasPassword, host, port, database, query), nil
	}
	if driver != "mysql" {
		return sqlTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
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
	return sqlTarget{Driver: "mysql", Dialect: "mysql", DSN: dsn, SafeURL: safeURL, Database: database}, nil
}

func postgresTargetFromParts(user, password string, hasPassword bool, host, port, database, query string) sqlTarget {
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
	return sqlTarget{Driver: "pgx", Dialect: "postgres", DSN: dsn, SafeURL: safe.String(), Database: database}
}

func targetFromSQLiteURL(parsed *url.URL, rawURL, databaseOverride string) sqlTarget {
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
	return sqlTarget{Driver: "sqlite", Dialect: "sqlite", DSN: dsn, SafeURL: "sqlite://" + dsn, Database: database}
}

func parseOptionalURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func normalizeDriver(driver, scheme string) string {
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

func defaultUser(driver string) string {
	switch normalizeDriver(driver, "") {
	case "mysql":
		return "root"
	case "pgx":
		return "postgres"
	default:
		return ""
	}
}

func supportedDriver(driver string) bool {
	switch normalizeDriver(driver, "") {
	case "mysql", "pgx", "sqlite":
		return true
	default:
		return false
	}
}

func dialectName(driver string) string {
	switch normalizeDriver(driver, "") {
	case "pgx":
		return "postgres"
	default:
		return normalizeDriver(driver, "")
	}
}

func detectDSNDriver(dsn string) string {
	switch {
	case looksLikeMySQLDriverDSN(dsn):
		return "mysql"
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"), strings.Contains(dsn, "host="):
		return "pgx"
	default:
		return ""
	}
}

func safeDSN(driver, dsn string) string {
	switch normalizeDriver(driver, "") {
	case "mysql", "pgx":
		return redactDSN(dsn)
	case "sqlite":
		return "sqlite://" + dsn
	default:
		return dsn
	}
}

func scanRows(rows *stdsql.Rows, maxRows int) ([]map[string]any, []string, bool, error) {
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

func readOnlyQuery(query string) bool {
	trimmed := strings.TrimSpace(strings.TrimLeft(query, "("))
	first := strings.ToLower(strings.Fields(trimmed)[0])
	switch first {
	case "select", "show", "describe", "desc", "explain", "with":
		return true
	default:
		return false
	}
}

func parseDurationDefault(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func defaultOpen(driverName, dsn string) (Queryer, error) {
	return stdsql.Open(driverName, dsn)
}

func looksLikeMySQLDriverDSN(value string) bool {
	return strings.Contains(value, "@tcp(") || strings.Contains(value, "@unix(")
}

func databaseFromDriverDSN(dsn string) string {
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

func redactDSN(dsn string) string {
	if at := strings.Index(dsn, "@"); at > 0 {
		userInfo := dsn[:at]
		if colon := strings.Index(userInfo, ":"); colon >= 0 {
			return userInfo[:colon] + ":xxxxx" + dsn[at:]
		}
	}
	return dsn
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func sqlRowID(out QueryOutput, query string, index int, row map[string]any) string {
	data, _ := json.Marshal(row)
	sum := sha1.Sum([]byte(out.Driver + "\x00" + out.Database + "\x00" + out.EndpointURL + "\x00" + query + "\x00" + strconv.Itoa(index) + "\x00" + string(data)))
	return hex.EncodeToString(sum[:])
}

func sqlRowTitle(index int, row map[string]any, columns []string) string {
	for _, column := range columns {
		if value, ok := row[column]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" {
				return text
			}
		}
	}
	return fmt.Sprintf("row %d", index+1)
}
