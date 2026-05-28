package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-dex/core"
	"github.com/fluxplane/fluxplane-dex/protocol"
)

type Runner struct {
	Marketplace Marketplace
	State       State
	DevPlugins  map[string]string
	WorkDir     string
	Timeout     time.Duration
	HostCommand string
}

func (r Runner) Invoke(ctx context.Context, pluginName, command string, payload any) (protocol.Response, error) {
	return r.InvokeInstance(ctx, pluginName, DefaultInstance, command, payload)
}

func (r Runner) InvokeInstance(ctx context.Context, pluginName, instance, command string, payload any) (protocol.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	entry, ok := r.Marketplace.Resolve(pluginName)
	if !ok {
		return protocol.Response{}, fmt.Errorf("unknown plugin %q", pluginName)
	}
	req, err := protocol.NewRequest(command, entry.Name, payload)
	if err != nil {
		return protocol.Response{}, err
	}
	req.Instance = NormalizeInstance(instance)
	if command == protocol.CommandOperationsCall || command == protocol.CommandOperationsBatch {
		if err := r.resolveEndpointRefs(&req); err != nil {
			return protocol.Response{}, err
		}
	}
	if isDatasourceCommand(command) {
		if err := r.resolveDatasourceEndpointRef(&req); err != nil {
			return protocol.Response{}, err
		}
	}
	if isDatasourceCommand(command) {
		resp, ok, err := (hostIndexDatasource{state: r.State, plugin: entry.Name, instance: req.Instance}).Response(command, payload)
		if err != nil {
			return protocol.Response{}, err
		}
		if ok {
			return resp, nil
		}
	}
	if isBuiltinPlugin(entry) {
		resp, err := r.invokeBuiltin(ctx, entry, req)
		if err != nil {
			return protocol.Response{}, err
		}
		return r.enrichDatasourceResponse(ctx, entry.Name, req.Instance, command, payload, resp)
	}
	if command == protocol.CommandOperationsCall || command == protocol.CommandOperationsBatch {
		operations, purposes := r.operationGrantScope(ctx, entry.Name, payload)
		grant, err := r.State.CreateGrant(entry.Name, req.Instance, operations, purposes, 5*time.Minute)
		if err != nil {
			return protocol.Response{}, err
		}
		req.Grant = grant.Token
	}
	if command == protocol.CommandDatasourcesSearch || command == protocol.CommandDatasourcesGet || command == protocol.CommandDatasourcesLookup {
		operations, purposes := r.datasourceGrantScope(ctx, entry.Name, command, payload)
		if len(purposes) > 0 {
			grant, err := r.State.CreateGrant(entry.Name, req.Instance, operations, purposes, 5*time.Minute)
			if err != nil {
				return protocol.Response{}, err
			}
			req.Grant = grant.Token
		}
	}
	resp, err := r.invokeRequest(ctx, entry, req)
	if err != nil {
		return protocol.Response{}, err
	}
	return r.enrichDatasourceResponse(ctx, entry.Name, req.Instance, command, payload, resp)
}

func (r Runner) resolveEndpointRefs(req *protocol.Request) error {
	if req == nil || len(req.Payload) == 0 {
		return nil
	}
	switch req.Command {
	case protocol.CommandOperationsCall:
		var call protocol.OperationCall
		if err := json.Unmarshal(req.Payload, &call); err != nil {
			return nil
		}
		changed, err := r.resolveOperationEndpointRef(&call)
		if err != nil {
			return err
		}
		if changed {
			raw, err := json.Marshal(call)
			if err != nil {
				return err
			}
			req.Payload = raw
		}
	case protocol.CommandOperationsBatch:
		var batch protocol.OperationBatch
		if err := json.Unmarshal(req.Payload, &batch); err != nil {
			return nil
		}
		changed := false
		for i := range batch.Calls {
			callChanged, err := r.resolveOperationEndpointRef(&batch.Calls[i])
			if err != nil {
				return err
			}
			changed = changed || callChanged
		}
		if changed {
			raw, err := json.Marshal(batch)
			if err != nil {
				return err
			}
			req.Payload = raw
		}
	}
	return nil
}

func (r Runner) resolveOperationEndpointRef(call *protocol.OperationCall) (bool, error) {
	if call == nil || len(call.Input) == 0 {
		return false, nil
	}
	var input map[string]any
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return false, nil
	}
	changed, err := r.resolveEndpointRefInput(input)
	if err != nil || !changed {
		return changed, err
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return false, err
	}
	call.Input = raw
	return true, nil
}

func (r Runner) resolveDatasourceEndpointRef(req *protocol.Request) error {
	if req == nil || len(req.Payload) == 0 {
		return nil
	}
	var input map[string]any
	if err := json.Unmarshal(req.Payload, &input); err != nil {
		return nil
	}
	changed, err := r.resolveEndpointRefInput(input)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req.Payload = raw
	return nil
}

func (r Runner) resolveEndpointRefInput(input map[string]any) (bool, error) {
	if len(input) == 0 {
		return false, nil
	}
	ref, _ := input["endpoint_ref"].(string)
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false, nil
	}
	if value, _ := input["url"].(string); strings.TrimSpace(value) != "" {
		return false, nil
	}
	endpoint, ok, err := r.State.GetEndpoint(ref)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("unknown endpoint_ref %q", ref)
	}
	input["url"] = endpoint.URL
	if endpoint.CredentialRef != "" {
		input["credential_ref"] = endpoint.CredentialRef
	}
	if _, ok := input["endpoint_product"]; !ok && endpoint.Product != "" {
		input["endpoint_product"] = endpoint.Product
	}
	return true, nil
}

type IndexBuildResult struct {
	Plugin    string    `json:"plugin"`
	Instance  string    `json:"instance"`
	Index     string    `json:"index,omitempty"`
	Indexes   []string  `json:"indexes,omitempty"`
	Records   int       `json:"records"`
	Stored    bool      `json:"stored"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r Runner) BuildIndex(ctx context.Context, pluginName, instance string, input any) (IndexBuildResult, error) {
	entry, ok := r.Marketplace.Resolve(pluginName)
	if !ok {
		return IndexBuildResult{}, fmt.Errorf("unknown plugin %q", pluginName)
	}
	operation := entry.Name + ".index.build"
	inputRaw, err := json.Marshal(input)
	if err != nil {
		return IndexBuildResult{}, err
	}
	resp, err := r.InvokeInstance(ctx, entry.Name, instance, protocol.CommandOperationsCall, protocol.OperationCall{Name: operation, Input: inputRaw})
	if err != nil {
		return IndexBuildResult{}, err
	}
	var result struct {
		Index   string `json:"index"`
		Records []json.RawMessage
		Indexes []struct {
			Index    string            `json:"index"`
			Records  []json.RawMessage `json:"records"`
			Metadata json.RawMessage   `json:"metadata,omitempty"`
		} `json:"indexes"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return IndexBuildResult{}, err
	}
	indexes := result.Indexes
	if len(indexes) == 0 {
		indexName := result.Index
		if strings.TrimSpace(indexName) == "" {
			indexName = defaultIndexName(ctx, r, entry.Name)
		}
		indexes = append(indexes, struct {
			Index    string            `json:"index"`
			Records  []json.RawMessage `json:"records"`
			Metadata json.RawMessage   `json:"metadata,omitempty"`
		}{Index: indexName, Records: result.Records})
	}
	out := IndexBuildResult{Plugin: entry.Name, Instance: NormalizeInstance(instance), Stored: true}
	for _, index := range indexes {
		if strings.TrimSpace(index.Index) == "" {
			return IndexBuildResult{}, fmt.Errorf("index build result did not include index name")
		}
		snapshot, err := r.State.SaveIndexRecordsWithMetadata(entry.Name, instance, index.Index, index.Records, index.Metadata)
		if err != nil {
			return IndexBuildResult{}, err
		}
		out.Indexes = append(out.Indexes, snapshot.Index)
		out.Records += len(snapshot.Records)
		if snapshot.UpdatedAt.After(out.UpdatedAt) {
			out.UpdatedAt = snapshot.UpdatedAt
		}
	}
	if len(out.Indexes) == 1 {
		out.Index = out.Indexes[0]
	}
	if err := r.State.ActivatePlugin(entry); err != nil {
		return IndexBuildResult{}, err
	}
	return out, nil
}

func (r Runner) invokeRequest(ctx context.Context, entry core.PluginEntry, req protocol.Request) (protocol.Response, error) {
	cmd, err := r.command(ctx, entry)
	if err != nil {
		return protocol.Response{}, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return protocol.Response{}, err
	}
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = r.pluginEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.Len() > 0 {
			resp, decodeErr := decodeResponse(stdout.Bytes())
			if resp.Protocol == protocol.Version {
				return resp, decodeErr
			}
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return protocol.Response{}, fmt.Errorf("run plugin %s: %s", entry.Name, msg)
	}
	return decodeResponse(stdout.Bytes())
}

func (r Runner) pluginEnv() []string {
	allowed := []string{
		"PATH",
		"HOME",
		"USER",
		"TMPDIR",
		"TEMP",
		"TMP",
		"GOCACHE",
		"GOPATH",
		"GOMODCACHE",
		"GOENV",
		"GOROOT",
		"GOPROXY",
		"GOSUMDB",
		"GONOSUMDB",
		"GOPRIVATE",
		"GONOPROXY",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"NO_PROXY",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
		"XDG_CACHE_HOME",
	}
	env := make([]string, 0, len(allowed)+2)
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	env = append(env, "DEX_HOST_CMD="+r.hostCommand())
	if strings.TrimSpace(r.State.Home) != "" {
		env = append(env, "DEX_HOME="+r.State.Home)
	}
	return env
}

func (r Runner) OperationBatch(ctx context.Context, pluginName, instance string, calls []protocol.OperationCall) (protocol.OperationBatchResult, error) {
	resp, err := r.InvokeInstance(ctx, pluginName, instance, protocol.CommandOperationsBatch, protocol.OperationBatch{Calls: calls})
	if err != nil {
		return protocol.OperationBatchResult{}, err
	}
	var result protocol.OperationBatchResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return protocol.OperationBatchResult{}, err
	}
	return result, nil
}

func (r Runner) Install(ctx context.Context, name string) error {
	_, err := r.InstallPlugin(ctx, name)
	return err
}

func (r Runner) InstallPlugin(ctx context.Context, name string) (InstalledPlugin, error) {
	return r.installPlugin(ctx, name, true)
}

func (r Runner) UpgradePlugin(ctx context.Context, name string) (InstalledPlugin, error) {
	installed, ok, err := r.State.InstalledPlugin(name)
	if err != nil {
		return InstalledPlugin{}, err
	}
	activated := true
	if ok {
		activated = installed.Activated
	}
	return r.installPlugin(ctx, name, activated)
}

func (r Runner) installPlugin(ctx context.Context, name string, activated bool) (InstalledPlugin, error) {
	entry, ok := r.Marketplace.Resolve(name)
	if !ok {
		return InstalledPlugin{}, fmt.Errorf("unknown plugin %q", name)
	}
	if strings.TrimSpace(entry.GoInstall) == "" {
		return InstalledPlugin{}, fmt.Errorf("plugin %q has no go_install target", entry.Name)
	}
	if strings.TrimSpace(entry.Binary) == "" {
		return InstalledPlugin{}, fmt.Errorf("plugin %q has no binary name", entry.Name)
	}
	path := r.State.PluginBinaryPath(executableName(entry.Binary))
	if pluginDir, ok := localPluginPath(r.WorkDir, entry.LocalPath); ok {
		if err := InstallLocalGoTarget(ctx, pluginDir, executableName(entry.Binary), r.State.PluginBinDir()); err != nil {
			return InstalledPlugin{}, err
		}
		if err := r.State.SaveInstalledPluginVersionActivated(entry, true, path, "local", activated); err != nil {
			return InstalledPlugin{}, err
		}
		installed, _, err := r.State.InstalledPlugin(entry.Name)
		return installed, err
	}
	if strings.TrimSpace(entry.GoInstall) == "" {
		return InstalledPlugin{}, fmt.Errorf("plugin %q has no go_install target", entry.Name)
	}
	version := ""
	if info, err := ResolveGoModuleVersion(ctx, entry.GoInstall, "latest"); err == nil {
		version = info.Version
	}
	if err := InstallGoTarget(ctx, entry.GoInstall, r.State.PluginBinDir()); err != nil {
		return InstalledPlugin{}, err
	}
	if err := r.State.SaveInstalledPluginVersionActivated(entry, true, path, version, activated); err != nil {
		return InstalledPlugin{}, err
	}
	installed, _, err := r.State.InstalledPlugin(entry.Name)
	return installed, err
}

func (r Runner) command(ctx context.Context, entry core.PluginEntry) (*exec.Cmd, error) {
	if path := strings.TrimSpace(r.DevPlugins[entry.Name]); path != "" {
		return goRunCommand(ctx, path, entry.Binary), nil
	}
	if path, ok, err := r.installedPluginCommandPath(entry); err != nil {
		return nil, err
	} else if ok {
		return exec.CommandContext(ctx, path), nil
	}
	if path, ok := localPluginPath(r.WorkDir, entry.LocalPath); ok {
		return goRunCommand(ctx, path, entry.Binary), nil
	}
	if binary, err := exec.LookPath(entry.Binary); err == nil {
		return exec.CommandContext(ctx, binary), nil
	}
	return nil, fmt.Errorf("plugin %q is not installed; run dex plugin install %s", entry.Name, entry.Name)
}

func (r Runner) installedPluginCommandPath(entry core.PluginEntry) (string, bool, error) {
	installed, ok, err := r.State.InstalledPlugin(entry.Name)
	if err != nil || !ok {
		return "", false, err
	}
	for _, candidate := range installedPluginCommandCandidates(r.State, installed, entry) {
		if isExecutableFile(candidate) {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

func installedPluginCommandCandidates(state State, installed InstalledPlugin, entry core.PluginEntry) []string {
	var candidates []string
	if path := strings.TrimSpace(installed.Path); path != "" {
		candidates = append(candidates, path)
	}
	for _, binary := range []string{installed.Binary, entry.Binary} {
		binary = strings.TrimSpace(binary)
		if binary == "" {
			continue
		}
		if filepath.IsAbs(binary) {
			candidates = append(candidates, binary)
			continue
		}
		candidates = append(candidates, state.PluginBinaryPath(executableName(binary)))
	}
	return dedupeStrings(candidates)
}

func isExecutableFile(path string) bool {
	stat, err := os.Stat(path)
	if err != nil || stat.IsDir() {
		return false
	}
	if goruntime.GOOS == "windows" {
		return true
	}
	return stat.Mode()&0o111 != 0
}

func executableName(binary string) string {
	binary = strings.TrimSpace(binary)
	if goruntime.GOOS == "windows" && strings.TrimSpace(filepath.Ext(binary)) == "" {
		return binary + ".exe"
	}
	return binary
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (r Runner) operationGrantScope(ctx context.Context, plugin string, payload any) ([]string, []SecretPurpose) {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil {
		return nil, nil
	}
	byName := map[string]core.OperationSpec{}
	for _, op := range manifest.Operations {
		byName[op.Name] = op
	}
	authEnv := manifestAuthEnv(manifest)
	var operations []string
	purposeByName := map[string]SecretPurpose{}
	for _, call := range operationCalls(payload) {
		operations = append(operations, call.Name)
		if spec, ok := byName[call.Name]; ok {
			for _, purpose := range spec.SecretPurposes {
				if purpose == "" {
					continue
				}
				purposeByName[purpose] = SecretPurpose{Name: purpose, Env: authEnv[purpose]}
			}
		}
	}
	var purposes []SecretPurpose
	for _, purpose := range purposeByName {
		purposes = append(purposes, purpose)
	}
	return operations, purposes
}

func (r Runner) datasourceGrantScope(ctx context.Context, plugin, command string, payload any) ([]string, []SecretPurpose) {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil {
		return nil, nil
	}
	authEnv := manifestAuthEnv(manifest)
	capability := datasourceCommandCapability(command)
	datasourceName := ""
	entity := ""
	switch command {
	case protocol.CommandDatasourcesSearch:
		options := searchPayload(payload)
		datasourceName = options.Datasource
		entity = options.Entity
	case protocol.CommandDatasourcesLookup:
		options := lookupPayload(payload)
		datasourceName = options.Datasource
		entity = options.Entity
	case protocol.CommandDatasourcesGet:
		datasourceName = getPayloadDatasource(payload)
	}
	purposeByName := map[string]SecretPurpose{}
	for _, datasource := range manifest.Datasources {
		if datasourceName != "" && datasource.Name != datasourceName {
			continue
		}
		if entity != "" && datasource.Entity != entity {
			continue
		}
		if capability != "" && !containsString(datasource.Capabilities, capability) {
			continue
		}
		for _, purpose := range datasource.SecretPurposes {
			if strings.TrimSpace(purpose) == "" {
				continue
			}
			purposeByName[purpose] = SecretPurpose{Name: purpose, Env: authEnv[purpose]}
		}
	}
	var purposes []SecretPurpose
	for _, purpose := range purposeByName {
		purposes = append(purposes, purpose)
	}
	return []string{command}, purposes
}

func manifestAuthEnv(manifest core.PluginManifest) map[string][]string {
	authEnv := map[string][]string{}
	for _, method := range manifest.Auth {
		for _, field := range method.Fields {
			if field.Name == "" {
				continue
			}
			if len(field.Env) > 0 {
				authEnv[field.Name] = append(authEnv[field.Name], field.Env...)
			} else {
				authEnv[field.Name] = append(authEnv[field.Name], method.Env...)
			}
		}
	}
	return authEnv
}

func datasourceCommandCapability(command string) string {
	switch command {
	case protocol.CommandDatasourcesSearch:
		return "search"
	case protocol.CommandDatasourcesGet:
		return "get"
	case protocol.CommandDatasourcesLookup:
		return "lookup"
	default:
		return ""
	}
}

func containsString(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func (r Runner) manifest(ctx context.Context, plugin string) (core.PluginManifest, error) {
	entry, ok := r.Marketplace.Resolve(plugin)
	if !ok {
		return core.PluginManifest{}, fmt.Errorf("unknown plugin %q", plugin)
	}
	req, err := protocol.NewRequest(protocol.CommandManifest, entry.Name, nil)
	if err != nil {
		return core.PluginManifest{}, err
	}
	req.Instance = DefaultInstance
	var resp protocol.Response
	if isBuiltinPlugin(entry) {
		resp, err = r.invokeBuiltin(ctx, entry, req)
	} else {
		resp, err = r.invokeRequest(ctx, entry, req)
	}
	if err != nil {
		return core.PluginManifest{}, err
	}
	var manifest core.PluginManifest
	if err := json.Unmarshal(resp.Result, &manifest); err != nil {
		return core.PluginManifest{}, err
	}
	return manifest, nil
}

func defaultIndexName(ctx context.Context, r Runner, plugin string) string {
	manifest, err := r.manifest(ctx, plugin)
	if err != nil || len(manifest.Indexes) == 0 {
		return ""
	}
	return manifest.Indexes[0].Name
}

func searchPayload(payload any) SearchOptions {
	data, err := json.Marshal(payload)
	if err != nil {
		return SearchOptions{Limit: 20}
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return SearchOptions{Limit: 20}
	}
	limit := 20
	switch value := input["limit"].(type) {
	case float64:
		limit = int(value)
	case int:
		limit = value
	}
	query, _ := input["query"].(string)
	datasource, _ := input["datasource"].(string)
	entity, _ := input["entity"].(string)
	return SearchOptions{Datasource: strings.TrimSpace(datasource), Query: strings.TrimSpace(query), Limit: limit, Entity: strings.TrimSpace(entity)}
}

func lookupPayload(payload any) LookupOptions {
	data, err := json.Marshal(payload)
	if err != nil {
		return LookupOptions{Limit: 20}
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return LookupOptions{Limit: 20}
	}
	limit := 20
	switch value := input["limit"].(type) {
	case float64:
		limit = int(value)
	case int:
		limit = value
	}
	text := firstPayloadString(input, "text", "query", "q")
	datasource := firstPayloadString(input, "datasource")
	entity := firstPayloadString(input, "entity")
	var terms []string
	if rawTerms, ok := input["terms"].([]any); ok {
		for _, term := range rawTerms {
			if text, ok := term.(string); ok && strings.TrimSpace(text) != "" {
				terms = append(terms, strings.TrimSpace(text))
			}
		}
	}
	if term := firstPayloadString(input, "term", "id", "ref", "url"); term != "" {
		terms = append(terms, term)
	}
	return LookupOptions{Datasource: datasource, Text: strings.TrimSpace(text), Terms: terms, Limit: limit, Entity: strings.TrimSpace(entity)}
}

func getPayloadDatasource(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}
	return firstPayloadString(input, "datasource")
}

func getPayloadID(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		return ""
	}
	for _, key := range []string{"id", "ref", "key"} {
		switch value := input[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case float64:
			return fmt.Sprintf("%.0f", value)
		}
	}
	return ""
}

func firstPayloadString(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := input[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func operationCalls(payload any) []protocol.OperationCall {
	switch value := payload.(type) {
	case protocol.OperationCall:
		return []protocol.OperationCall{value}
	case protocol.OperationBatch:
		return value.Calls
	default:
		data, err := json.Marshal(payload)
		if err != nil {
			return nil
		}
		var call protocol.OperationCall
		if err := json.Unmarshal(data, &call); err == nil && call.Name != "" {
			return []protocol.OperationCall{call}
		}
		var batch protocol.OperationBatch
		if err := json.Unmarshal(data, &batch); err == nil {
			return batch.Calls
		}
		return nil
	}
}

func (r Runner) hostCommand() string {
	if strings.TrimSpace(r.HostCommand) != "" {
		return r.HostCommand
	}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		return exe
	}
	return "dex"
}

func goRunCommand(ctx context.Context, dir, binary string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/"+binary)
	cmd.Dir = dir
	return cmd
}

func localPluginPath(workDir, localPath string) (string, bool) {
	if strings.TrimSpace(localPath) == "" {
		return "", false
	}
	if filepath.IsAbs(localPath) {
		if stat, err := os.Stat(localPath); err == nil && stat.IsDir() {
			return localPath, true
		}
		return "", false
	}
	candidates := []string{}
	if strings.TrimSpace(workDir) != "" {
		candidates = append(candidates, filepath.Join(workDir, localPath))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, localPath))
	}
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func decodeResponse(data []byte) (protocol.Response, error) {
	var resp protocol.Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, fmt.Errorf("decode plugin response: %w", err)
	}
	if resp.Protocol != protocol.Version {
		return resp, fmt.Errorf("plugin protocol mismatch: %s", resp.Protocol)
	}
	if !resp.OK && resp.Error != nil {
		return resp, fmt.Errorf("%s", resp.Error.Message)
	}
	return resp, nil
}
