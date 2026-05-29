package kuberneteshost

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientkubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func Contexts() (ClusterListResult, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return ClusterListResult{}, err
	}
	current := strings.TrimSpace(config.CurrentContext)
	contexts := make([]ClusterContext, 0, len(config.Contexts))
	for name, ctx := range config.Contexts {
		contexts = append(contexts, ClusterContext{Name: name, Current: name == current, Cluster: ctx.Cluster, User: ctx.AuthInfo})
	}
	sort.Slice(contexts, func(i, j int) bool { return contexts[i].Name < contexts[j].Name })
	return ClusterListResult{Contexts: contexts}, nil
}

func ClusterProbe(ctx context.Context, input ClusterTestInput) (ClusterTestResult, error) {
	contextName := clusterContextFromTestInput(input)
	start := time.Now()
	clientset, _, err := kubernetesClientWithTimeout(EndpointDiscoverInput{Context: contextName}, 10*time.Second)
	if err != nil {
		return ClusterTestResult{}, err
	}
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return ClusterTestResult{}, err
	}
	out := ClusterTestResult{Context: contextName, OK: true, DurationMS: time.Since(start).Milliseconds()}
	if version != nil {
		out.ServerVersion = version.GitVersion
		out.Platform = version.Platform
	}
	return out, nil
}

func Namespaces(ctx context.Context, input InventoryInput) ([]corev1.Namespace, error) {
	clientset, _, err := kubernetesClient(endpointInputFromInventory(input))
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func Services(ctx context.Context, input EndpointDiscoverInput) ([]corev1.Service, error) {
	clientset, namespace, err := kubernetesClient(input)
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func Ingresses(ctx context.Context, input EndpointDiscoverInput) ([]networkingv1.Ingress, error) {
	clientset, namespace, err := kubernetesClient(input)
	if err != nil {
		return nil, err
	}
	list, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func Pods(ctx context.Context, input InventoryInput) ([]corev1.Pod, error) {
	clientset, namespace, err := kubernetesClient(endpointInputFromInventory(input))
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func Deployments(ctx context.Context, input InventoryInput) ([]appsv1.Deployment, error) {
	clientset, namespace, err := kubernetesClient(endpointInputFromInventory(input))
	if err != nil {
		return nil, err
	}
	list, err := clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func PodLogs(ctx context.Context, input PodLogsInput) (PodLogsResult, error) {
	namespace := strings.TrimSpace(input.Namespace)
	name := strings.TrimSpace(input.Name)
	if namespace == "" {
		return PodLogsResult{}, fmt.Errorf("namespace is required")
	}
	if name == "" {
		return PodLogsResult{}, fmt.Errorf("name is required")
	}
	bounds, err := podLogBounds(input)
	if err != nil {
		return PodLogsResult{}, err
	}
	clientset, _, err := kubernetesClient(EndpointDiscoverInput{
		Context:   firstNonEmpty(input.Context, clusterContextFromEndpointURL(input.URL)),
		Namespace: namespace,
	})
	if err != nil {
		return PodLogsResult{}, err
	}
	options := &corev1.PodLogOptions{
		Container:  strings.TrimSpace(input.Container),
		Previous:   input.Previous,
		Timestamps: input.Timestamps || bounds.Until != nil,
	}
	if bounds.TailLines != nil {
		options.TailLines = bounds.TailLines
	}
	if bounds.LimitBytes != nil {
		options.LimitBytes = bounds.LimitBytes
	}
	if bounds.SinceSeconds != nil {
		options.SinceSeconds = bounds.SinceSeconds
	}
	if bounds.SinceTime != nil {
		options.SinceTime = bounds.SinceTime
	}
	raw, err := clientset.CoreV1().Pods(namespace).GetLogs(name, options).DoRaw(ctx)
	if err != nil {
		return PodLogsResult{}, err
	}
	text := filterPodLogText(strings.TrimRight(string(raw), "\n"), bounds, input.Timestamps)
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	return PodLogsResult{
		Namespace:  namespace,
		Name:       name,
		Container:  strings.TrimSpace(input.Container),
		Lines:      lines,
		Text:       text,
		LineCount:  len(lines),
		TailLines:  valueOrZero(bounds.TailLines),
		LimitBytes: valueOrZero(bounds.LimitBytes),
		Since:      strings.TrimSpace(input.Since),
		Until:      strings.TrimSpace(input.Until),
		Previous:   input.Previous,
		Timestamps: input.Timestamps,
	}, nil
}

func Secrets(ctx context.Context, input EndpointDiscoverInput) ([]corev1.Secret, error) {
	clientset, namespace, err := kubernetesClient(input)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return []corev1.Secret{*secret}, nil
	}
	list, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func ConfigMaps(ctx context.Context, input EndpointDiscoverInput) ([]corev1.ConfigMap, error) {
	clientset, namespace, err := kubernetesClient(input)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return []corev1.ConfigMap{*configMap}, nil
	}
	list, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func PortForwardStart(ctx context.Context, input PortForwardStartInput) (PortForwardResult, error) {
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		return PortForwardResult{}, fmt.Errorf("namespace is required")
	}
	resource := normalizedPortForwardResource(input)
	if resource == "" {
		return PortForwardResult{}, fmt.Errorf("resource or name is required")
	}
	remotePort := input.RemotePort
	if remotePort <= 0 {
		return PortForwardResult{}, fmt.Errorf("remote_port is required")
	}
	localPort := input.LocalPort
	if localPort <= 0 {
		port, err := availableLocalPort()
		if err != nil {
			return PortForwardResult{}, err
		}
		localPort = port
	}
	address := strings.TrimSpace(input.Address)
	if address == "" {
		address = "127.0.0.1"
	}
	duration := input.DurationSeconds
	if duration <= 0 {
		duration = 3600
	}
	if duration > 8*3600 {
		duration = 8 * 3600
	}
	contextName := firstNonEmpty(input.Context, clusterContextFromEndpointURL(input.URL))
	id := portForwardID(namespace, resource, localPort, remotePort)
	dir, err := portForwardStateDir()
	if err != nil {
		return PortForwardResult{}, err
	}
	logPath := filepath.Join(dir, id+".log")
	recordPath := filepath.Join(dir, id+".json")
	portArg := strconv.Itoa(localPort) + ":" + strconv.Itoa(remotePort)
	args := []string{}
	if contextName != "" {
		args = append(args, "--context", contextName)
	}
	args = append(args, "-n", namespace, "port-forward", resource, portArg, "--address", address)
	shell := "kubectl " + shellJoin(args) + " >>" + shellQuote(logPath) + " 2>&1 & child=$!; (sleep " + strconv.Itoa(duration) + "; kill $child >/dev/null 2>&1) & wait $child"
	cmd := exec.CommandContext(ctx, "sh", "-c", shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return PortForwardResult{}, err
	}
	if err := waitForPortForward(address, localPort, cmd.Process.Pid, logPath); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return PortForwardResult{}, err
	}
	result := PortForwardResult{
		ID:              id,
		EndpointRef:     strings.TrimSpace(input.EndpointRef),
		Context:         contextName,
		Namespace:       namespace,
		Resource:        resource,
		Address:         address,
		LocalPort:       localPort,
		RemotePort:      remotePort,
		LocalURL:        "http://" + net.JoinHostPort(address, strconv.Itoa(localPort)),
		PID:             cmd.Process.Pid,
		ProcessGroup:    cmd.Process.Pid,
		DurationSeconds: duration,
		ExpiresAt:       time.Now().UTC().Add(time.Duration(duration) * time.Second),
		LogPath:         logPath,
		Command:         append([]string{"kubectl"}, args...),
	}
	if err := writePortForwardRecord(recordPath, result); err != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return PortForwardResult{}, err
	}
	return result, nil
}

func PortForwardStop(_ context.Context, input PortForwardStopInput) (PortForwardStopResult, error) {
	result := PortForwardStopResult{ID: strings.TrimSpace(input.ID)}
	processGroup := input.ProcessGroup
	pid := input.PID
	if result.ID != "" {
		record, err := readPortForwardRecord(result.ID)
		if err != nil {
			return result, err
		}
		if processGroup <= 0 {
			processGroup = record.ProcessGroup
		}
		if pid <= 0 {
			pid = record.PID
		}
	}
	if processGroup > 0 {
		if err := syscall.Kill(-processGroup, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			result.Error = err.Error()
			return result, err
		}
		result.Stopped = true
		result.Signal = "SIGTERM"
		return result, nil
	}
	if pid > 0 {
		if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Stopped = true
		result.Signal = "SIGTERM"
		return result, nil
	}
	return result, fmt.Errorf("id, process_group, or pid is required")
}

type podLogBoundOptions struct {
	TailLines    *int64
	LimitBytes   *int64
	SinceSeconds *int64
	SinceTime    *metav1.Time
	Until        *time.Time
}

func podLogBounds(input PodLogsInput) (podLogBoundOptions, error) {
	var out podLogBoundOptions
	if input.TailLines > 0 {
		tailLines := input.TailLines
		out.TailLines = &tailLines
	}
	if input.LimitBytes > 0 {
		limitBytes := input.LimitBytes
		out.LimitBytes = &limitBytes
	}
	if out.TailLines == nil && out.LimitBytes == nil && strings.TrimSpace(input.Since) == "" && strings.TrimSpace(input.Until) == "" {
		tailLines := int64(100)
		out.TailLines = &tailLines
	}
	if since := strings.TrimSpace(input.Since); since != "" {
		if duration, err := time.ParseDuration(since); err == nil {
			seconds := int64(duration.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			out.SinceSeconds = &seconds
		} else {
			parsed, parseErr := time.Parse(time.RFC3339, since)
			if parseErr != nil {
				return out, fmt.Errorf("since must be a duration or RFC3339 timestamp")
			}
			out.SinceTime = &metav1.Time{Time: parsed}
		}
	}
	if until := strings.TrimSpace(input.Until); until != "" {
		parsed, err := time.Parse(time.RFC3339, until)
		if err != nil {
			return out, fmt.Errorf("until must be an RFC3339 timestamp")
		}
		out.Until = &parsed
	}
	return out, nil
}

func filterPodLogText(text string, bounds podLogBoundOptions, keepTimestamps bool) string {
	if text == "" || bounds.Until == nil {
		return text
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		timestamp, rest, ok := splitKubernetesLogTimestamp(line)
		if !ok {
			out = append(out, line)
			continue
		}
		if timestamp.After(*bounds.Until) {
			continue
		}
		if keepTimestamps {
			out = append(out, line)
		} else {
			out = append(out, rest)
		}
	}
	return strings.Join(out, "\n")
}

func splitKubernetesLogTimestamp(line string) (time.Time, string, bool) {
	head, rest, ok := strings.Cut(line, " ")
	if !ok {
		return time.Time{}, "", false
	}
	timestamp, err := time.Parse(time.RFC3339Nano, head)
	if err != nil {
		return time.Time{}, "", false
	}
	return timestamp, rest, true
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func endpointInputFromInventory(input InventoryInput) EndpointDiscoverInput {
	return EndpointDiscoverInput{
		Context:   firstNonEmpty(input.Context, clusterContextFromEndpointURL(input.URL)),
		Namespace: input.Namespace,
		Limit:     input.Limit,
	}
}

func clusterContextFromTestInput(input ClusterTestInput) string {
	if strings.TrimSpace(input.Context) != "" {
		return strings.TrimSpace(input.Context)
	}
	return clusterContextFromEndpointURL(input.URL)
}

func clusterContextFromEndpointURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	if parsed.Scheme != "kubernetes" && parsed.Scheme != "k8s" {
		return ""
	}
	if parsed.Host != "context" {
		return ""
	}
	return strings.Trim(strings.TrimSpace(parsed.Path), "/")
}

func normalizedPortForwardResource(input PortForwardStartInput) string {
	resource := strings.TrimSpace(input.Resource)
	if resource != "" {
		return resource
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ""
	}
	resourceType := strings.Trim(strings.ToLower(strings.TrimSpace(input.ResourceType)), "/")
	if resourceType == "" {
		resourceType = "service"
	}
	return resourceType + "/" + name
}

func availableLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("could not allocate local TCP port")
	}
	return addr.Port, nil
}

func portForwardID(namespace, resource string, localPort, remotePort int) string {
	sum := sha1.Sum([]byte(namespace + "\x00" + resource + "\x00" + strconv.Itoa(localPort) + "\x00" + strconv.Itoa(remotePort) + "\x00" + strconv.FormatInt(time.Now().UnixNano(), 10)))
	return "kpf-" + hex.EncodeToString(sum[:6])
}

func portForwardStateDir() (string, error) {
	root := filepath.Join(os.TempDir(), "fluxplane-dex", "kubernetes", "portforwards")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func writePortForwardRecord(path string, result PortForwardResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func readPortForwardRecord(id string) (PortForwardResult, error) {
	id = strings.TrimSpace(id)
	if !validPortForwardID(id) {
		return PortForwardResult{}, fmt.Errorf("invalid port-forward id %q", id)
	}
	dir, err := portForwardStateDir()
	if err != nil {
		return PortForwardResult{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, strings.TrimSpace(id)+".json"))
	if err != nil {
		return PortForwardResult{}, err
	}
	var result PortForwardResult
	if err := json.Unmarshal(data, &result); err != nil {
		return PortForwardResult{}, err
	}
	return result, nil
}

func validPortForwardID(id string) bool {
	if len(id) != len("kpf-")+12 || !strings.HasPrefix(id, "kpf-") {
		return false
	}
	for _, r := range id[len("kpf-"):] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func waitForPortForward(address string, port, pid int, logPath string) error {
	host := strings.TrimSpace(address)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	target := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", target, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if processExited(pid) {
			return fmt.Errorf("kubectl port-forward exited before %s became reachable: %s", target, tailFile(logPath, 2048))
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("kubectl port-forward did not become reachable at %s: %s", target, tailFile(logPath, 2048))
}

func processExited(pid int) bool {
	if pid <= 0 {
		return true
	}
	err := syscall.Kill(pid, 0)
	return errors.Is(err, syscall.ESRCH)
}

func tailFile(path string, max int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if max > 0 && len(data) > max {
		data = data[len(data)-max:]
	}
	return strings.TrimSpace(string(data))
}

func shellJoin(args []string) string {
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func kubernetesClient(input EndpointDiscoverInput) (*clientkubernetes.Clientset, string, error) {
	return kubernetesClientWithTimeout(input, 0)
}

func kubernetesClientWithTimeout(input EndpointDiscoverInput, timeout time.Duration) (*clientkubernetes.Clientset, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if strings.TrimSpace(input.Context) != "" {
		overrides.CurrentContext = strings.TrimSpace(input.Context)
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, "", err
	}
	if timeout > 0 {
		restConfig.Timeout = timeout
	}
	clientset, err := clientkubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}
	namespace := strings.TrimSpace(input.Namespace)
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	return clientset, namespace, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
