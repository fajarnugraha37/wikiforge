package openwiki

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/config"
)

type Runner interface {
	Run(ctx context.Context, workdir string, operation string, prompt string) (string, error)
	Check(ctx context.Context) error
}

type runLabelKey struct{}

const promptBridgePrefix = "WIKIFORGE_PROMPT_REF="

// WithRunLabel attaches a human-readable phase label used by ExecRunner for
// live child-process output and heartbeat messages.
func WithRunLabel(ctx context.Context, label string) context.Context {
	return context.WithValue(ctx, runLabelKey{}, strings.TrimSpace(label))
}

func runLabel(ctx context.Context) string {
	label, _ := ctx.Value(runLabelKey{}).(string)
	if strings.TrimSpace(label) == "" {
		return "openwiki"
	}
	return label
}

type ExecRunner struct {
	Config            config.OpenWikiConfig
	Out               io.Writer
	HeartbeatInterval time.Duration
	LiveOutput        bool
}

var consoleMu sync.Mutex

func (r ExecRunner) output() io.Writer {
	if r.Out == nil {
		return io.Discard
	}
	return r.Out
}

func writeConsole(w io.Writer, format string, args ...any) {
	if w == nil || w == io.Discard {
		return
	}
	consoleMu.Lock()
	defer consoleMu.Unlock()
	_, _ = fmt.Fprintf(w, format, args...)
}

func (r ExecRunner) Check(ctx context.Context) error {
	if _, err := exec.LookPath(r.Config.Command); err != nil {
		return fmt.Errorf("openwiki command %q not found: %w", r.Config.Command, err)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	args := append([]string{}, r.Config.Args...)
	args = append(args, "--help")
	cmd := exec.CommandContext(checkCtx, r.Config.Command, args...)
	cmd.Env = mergedEnv(r.Config.Environment)
	stderr := boundedOutput{max: 64 * 1024}
	cmd.Stdout = io.Discard
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if checkCtx.Err() != nil {
			return fmt.Errorf("OpenWiki help check timed out")
		}
		return fmt.Errorf("OpenWiki help check failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (r ExecRunner) Run(ctx context.Context, workdir string, operation string, prompt string) (string, error) {
	timeout := time.Duration(r.Config.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 60 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	absoluteWorkdir, err := canonicalWorkdir(workdir)
	if err != nil {
		return "", fmt.Errorf("resolve OpenWiki workdir: %w", err)
	}
	cliPrompt, promptPath, cleanupPrompt, err := externalizePrompt(absoluteWorkdir, prompt)
	if err != nil {
		return "", err
	}
	defer cleanupPrompt()

	args := append([]string{}, r.Config.Args...)
	switch operation {
	case "init":
		args = append(args, "--init", "--print", cliPrompt)
	case "update":
		args = append(args, "--update", "--print", cliPrompt)
	case "prompt":
		args = append(args, "--print", cliPrompt)
	case "discovery":
		args = append(args, "--print", cliPrompt)
	default:
		return "", fmt.Errorf("unsupported OpenWiki operation %q", operation)
	}
	if strings.TrimSpace(r.Config.ModelID) != "" {
		args = append(args, "--modelId", r.Config.ModelID)
	}

	cmd := exec.CommandContext(runCtx, r.Config.Command, args...)
	cmd.Dir = absoluteWorkdir
	cmd.Env = mergedEnv(r.Config.Environment)
	// Explicitly keep the process non-interactive. If a provider or package
	// attempts to prompt, it receives EOF instead of silently waiting forever.
	cmd.Stdin = nil

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("OpenWiki stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("OpenWiki stderr pipe: %w", err)
	}

	label := runLabel(ctx)
	out := r.output()
	heartbeat := r.HeartbeatInterval
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}
	live := r.LiveOutput
	// Preserve backwards compatibility for zero-value runners created outside
	// the CLI: an attached output writer implies live output.
	if r.Out != nil && r.Out != io.Discard {
		live = true
	}

	captureLimit := r.Config.MaxCaptureBytes
	if captureLimit <= 0 {
		captureLimit = 256 * 1024
	}
	stdoutLog, stderrLog, closeLogs, err := openRunLogs(r.Config.LogDirectory, label)
	if err != nil {
		return "", err
	}
	defer closeLogs()
	stdout := boundedOutput{max: captureLimit, full: stdoutLog}
	stderr := boundedOutput{max: captureLimit, full: stderrLog}
	var captureMu sync.Mutex
	var lastOutput atomic.Int64
	started := time.Now()
	lastOutput.Store(started.UnixNano())

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start OpenWiki: %w", err)
	}
	writeConsole(out, "[%s] OpenWiki process started pid=%d operation=%s timeout=%s\n", label, cmd.Process.Pid, operation, timeout)
	writeConsole(out, "[%s] prompt externalized path=%s bytes=%d cli-bytes=%d\n", label, promptPath, len([]byte(prompt)), len([]byte(cliPrompt)))

	var readers sync.WaitGroup
	readers.Add(2)
	go streamLines(stdoutPipe, "stdout", label, live, out, &stdout, &captureMu, &lastOutput, &readers)
	go streamLines(stderrPipe, "stderr", label, live, out, &stderr, &captureMu, &lastOutput, &readers)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				last := time.Unix(0, lastOutput.Load())
				writeConsole(out, "[%s] still running | elapsed=%s | quiet=%s | timeout=%s\n", label, compactDuration(time.Since(started)), compactDuration(time.Since(last)), timeout)
			}
		}
	}()

	waitErr := cmd.Wait()
	cancel()
	readers.Wait()
	<-done

	captureMu.Lock()
	stdoutText := stdout.String()
	stderrText := stderr.String()
	captureMu.Unlock()

	if waitErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return stdoutText, fmt.Errorf("OpenWiki timed out after %s; last stderr: %s", timeout, tail(stderrText, 4000))
		}
		if runCtx.Err() == context.Canceled && ctx.Err() != nil {
			return stdoutText, ctx.Err()
		}
		return stdoutText, fmt.Errorf("OpenWiki failed: %w\n%s", waitErr, tail(stderrText, 8000))
	}
	if looksLikeClarificationResponse(stdoutText) {
		writeConsole(out, "[%s] OpenWiki returned clarification instead of executing the phase; treating exit code 0 as failure\n", label)
		return stdoutText, fmt.Errorf("OpenWiki returned a clarification instead of executing the WikiForge task")
	}
	writeConsole(out, "[%s] OpenWiki process completed | elapsed=%s\n", label, compactDuration(time.Since(started)))
	return stdoutText, nil
}

// CheckPromptTransport verifies that a component work directory can host a
// temporary prompt and that the path exported to OpenWiki is an absolute
// virtual-repository path, not a host filesystem path.
func CheckPromptTransport(workdir string) error {
	cliPrompt, toolPath, cleanup, err := externalizePrompt(workdir, "WikiForge prompt transport preflight")
	if err != nil {
		return err
	}
	defer cleanup()
	if strings.ContainsAny(toolPath, "\"\r\n") {
		return fmt.Errorf("portable prompt path contains unsafe characters: %q", toolPath)
	}
	if strings.ContainsAny(cliPrompt, "\r\n") {
		return fmt.Errorf("prompt bridge must remain single-line for Windows command wrappers")
	}
	extracted, err := promptPathFromBridge(cliPrompt)
	if err != nil {
		return err
	}
	if extracted != toolPath {
		return fmt.Errorf("prompt bridge path mismatch: got %q want %q", extracted, toolPath)
	}
	absoluteWorkdir, err := canonicalWorkdir(workdir)
	if err != nil {
		return fmt.Errorf("resolve prompt workdir: %w", err)
	}
	native, err := promptHostPath(absoluteWorkdir, toolPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(native); err != nil {
		return fmt.Errorf("portable prompt path cannot be reopened: %w", err)
	}
	return nil
}

func canonicalWorkdir(workdir string) (string, error) {
	absolute, err := filepath.Abs(workdir)
	if err != nil {
		return "", err
	}
	absolute = filepath.Clean(absolute)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if os.IsNotExist(err) {
		return absolute, nil
	}
	return "", err
}

func externalizePrompt(workdir, prompt string) (cliPrompt, toolPath string, cleanup func(), err error) {
	if strings.TrimSpace(workdir) == "" {
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: workdir is empty")
	}
	absoluteWorkdir, err := canonicalWorkdir(workdir)
	if err != nil {
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: absolute workdir: %w", err)
	}
	openWikiRoot := filepath.Join(absoluteWorkdir, "openwiki")
	if err := os.MkdirAll(openWikiRoot, 0o755); err != nil {
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: create workdir: %w", err)
	}
	f, err := os.CreateTemp(openWikiRoot, ".wikiforge-prompt-*.md")
	if err != nil {
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: create temporary file: %w", err)
	}
	name, err := filepath.Abs(f.Name())
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: absolute temporary path: %w", err)
	}
	cleanup = func() { _ = os.Remove(name) }
	if _, err := io.WriteString(f, prompt); err != nil {
		_ = f.Close()
		cleanup()
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: write temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: close temporary file: %w", err)
	}
	toolPath = "/openwiki/" + filepath.ToSlash(filepath.Base(name))
	ref, err := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: toolPath})
	if err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("externalize OpenWiki prompt: encode bridge reference: %w", err)
	}
	// Keep this bridge strictly single-line. Windows npx.cmd/cmd.exe wrappers can
	// truncate or reinterpret embedded newlines even when Go supplied one argv
	// element, which previously hid the path from OpenWiki and caused it to ask
	// the user for clarification while still exiting successfully.
	cliPrompt = promptBridgePrefix + string(ref) + " This is a non-interactive WikiForge page task. Parse the JSON object after WIKIFORGE_PROMPT_REF, take only its path string value without quotation marks, and immediately use the filesystem read tool to read that exact absolute virtual UTF-8 file. The path is rooted at the repository virtual filesystem and begins with /openwiki/; do not convert it to a host path. Execute every instruction in that file now. Do not ask for clarification, do not search for another specification such as wikiforge.yaml, do not merely summarize the file, and do not modify, document, move, or delete the prompt file."
	return cliPrompt, toolPath, cleanup, nil
}

func promptHostPath(workdir, virtualPath string) (string, error) {
	const root = "/openwiki/"
	if !strings.HasPrefix(virtualPath, root) {
		return "", fmt.Errorf("prompt virtual path must start with %s: %q", root, virtualPath)
	}
	fileName := strings.TrimPrefix(virtualPath, root)
	if fileName == "" || strings.ContainsAny(fileName, `/\\`) || fileName == "." || fileName == ".." || strings.Contains(fileName, "..") {
		return "", fmt.Errorf("prompt virtual path is invalid: %q", virtualPath)
	}
	return filepath.Join(workdir, "openwiki", fileName), nil
}

func promptPathFromBridge(cliPrompt string) (string, error) {
	index := strings.Index(cliPrompt, promptBridgePrefix)
	if index < 0 {
		return "", fmt.Errorf("prompt bridge prefix is missing")
	}
	decoder := json.NewDecoder(strings.NewReader(cliPrompt[index+len(promptBridgePrefix):]))
	var ref struct {
		Path string `json:"path"`
	}
	if err := decoder.Decode(&ref); err != nil {
		return "", fmt.Errorf("decode prompt bridge reference: %w", err)
	}
	if strings.TrimSpace(ref.Path) == "" {
		return "", fmt.Errorf("prompt bridge path is empty")
	}
	return ref.Path, nil
}

func looksLikeClarificationResponse(output string) bool {
	text := strings.ToLower(strings.TrimSpace(output))
	if text == "" {
		return false
	}
	for _, marker := range []string{
		"could you clarify",
		"what would you like me to do",
		"please provide the absolute path",
		"do you have a file path",
		"i don't see a file path",
		"i'm not seeing a specific file path",
		"where is the file",
		"let me know what you need",
		"once i have the path",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// IsNonRetryableError identifies deterministic local invocation and path
// transport failures that cannot be healed by repeating the same model call.
func IsNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"the command line is too long",
		"filename or extension is too long",
		"path must be absolute",
		"portable prompt path",
		"externalize openwiki prompt",
		"unsupported openwiki operation",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func mergedEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

type boundedOutput struct {
	max       int
	data      []byte
	full      io.Writer
	truncated bool
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.full != nil {
		_, _ = b.full.Write(p)
	}
	if b.max <= 0 {
		return len(p), nil
	}
	if len(p) >= b.max {
		b.data = append(b.data[:0], p[len(p)-b.max:]...)
		b.truncated = true
		return len(p), nil
	}
	if len(b.data)+len(p) > b.max {
		remove := len(b.data) + len(p) - b.max
		b.data = append(b.data[:0], b.data[remove:]...)
		b.truncated = true
	}
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *boundedOutput) String() string {
	if !b.truncated {
		return string(b.data)
	}
	return "[output truncated; showing diagnostic tail]\n" + string(b.data)
}

func streamLines(r io.Reader, stream, label string, live bool, out io.Writer, capture *boundedOutput, captureMu *sync.Mutex, lastOutput *atomic.Int64, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	// Model/tool output can contain large JSON or Markdown lines.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		lastOutput.Store(time.Now().UnixNano())
		captureMu.Lock()
		_, _ = capture.Write([]byte(line + "\n"))
		captureMu.Unlock()
		if live {
			writeConsole(out, "[%s][%s] %s\n", label, stream, line)
		}
	}
	if err := scanner.Err(); err != nil {
		captureMu.Lock()
		_, _ = capture.Write([]byte(fmt.Sprintf("stream read error: %v\n", err)))
		captureMu.Unlock()
		writeConsole(out, "[%s][%s] stream read error: %v\n", label, stream, err)
	}
}

func openRunLogs(directory, label string) (io.Writer, io.Writer, func(), error) {
	if strings.TrimSpace(directory) == "" {
		return nil, nil, func() {}, nil
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, nil, func() {}, fmt.Errorf("create OpenWiki log directory: %w", err)
	}
	safe := strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "_").Replace(label)
	base := filepath.Join(directory, fmt.Sprintf("%s-%d", safe, time.Now().UnixNano()))
	stdout, err := os.Create(base + "-stdout.log")
	if err != nil {
		return nil, nil, func() {}, fmt.Errorf("create OpenWiki stdout log: %w", err)
	}
	stderr, err := os.Create(base + "-stderr.log")
	if err != nil {
		_ = stdout.Close()
		return nil, nil, func() {}, fmt.Errorf("create OpenWiki stderr log: %w", err)
	}
	return stdout, stderr, func() { _ = stdout.Close(); _ = stderr.Close() }, nil
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int((d % time.Hour) / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func tail(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return "..." + s[len(s)-max:]
}
