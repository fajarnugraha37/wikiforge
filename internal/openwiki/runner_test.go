package openwiki

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/example/wikiforge/internal/config"
)

func TestExecRunnerBuildsInitUpdateAndPromptCommands(t *testing.T) {
	for _, operation := range []string{"init", "update", "prompt"} {
		t.Run(operation, func(t *testing.T) {
			tmp := t.TempDir()
			capture := filepath.Join(tmp, "args.txt")
			promptCapture := filepath.Join(tmp, "prompt.txt")
			runner := ExecRunner{Config: config.OpenWikiConfig{
				Command:        os.Args[0],
				Args:           []string{"-test.run=TestOpenWikiHelperProcess", "--", "code"},
				ModelID:        "cheap-model",
				TimeoutMinutes: 1,
				Environment: map[string]string{
					"WIKIFORGE_HELPER_PROCESS":      "1",
					"WIKIFORGE_CAPTURE_PATH":        capture,
					"WIKIFORGE_CAPTURE_PROMPT_PATH": promptCapture,
				},
			}}
			workdir := t.TempDir()
			if _, err := runner.Run(context.Background(), workdir, operation, "phase prompt"); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			args := string(b)
			for _, required := range []string{"code", "--print", promptBridgePrefix, "--modelId", "cheap-model"} {
				if !strings.Contains(args, required) {
					t.Fatalf("operation %s missing %q in %q", operation, required, args)
				}
			}
			if operation == "init" && !strings.Contains(args, "--init") {
				t.Fatalf("init flag missing: %q", args)
			}
			if operation == "update" && !strings.Contains(args, "--update") {
				t.Fatalf("update flag missing: %q", args)
			}
			promptBytes, err := os.ReadFile(promptCapture)
			if err != nil {
				t.Fatal(err)
			}
			if string(promptBytes) != "phase prompt" {
				t.Fatalf("externalized prompt mismatch: %q", string(promptBytes))
			}
			matches, err := filepath.Glob(filepath.Join(workdir, ".wikiforge-prompt-*.md"))
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 0 {
				t.Fatalf("temporary prompt was not cleaned up: %v", matches)
			}
		})
	}
}

func TestExecRunnerExternalizesVeryLargePrompt(t *testing.T) {
	tmp := t.TempDir()
	capture := filepath.Join(tmp, "args.txt")
	promptCapture := filepath.Join(tmp, "prompt.txt")
	longPrompt := strings.Repeat("0123456789abcdef", 10000)
	runner := ExecRunner{Config: config.OpenWikiConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestOpenWikiHelperProcess", "--", "code"},
		TimeoutMinutes: 1,
		Environment: map[string]string{
			"WIKIFORGE_HELPER_PROCESS":      "1",
			"WIKIFORGE_CAPTURE_PATH":        capture,
			"WIKIFORGE_CAPTURE_PROMPT_PATH": promptCapture,
		},
	}}
	if _, err := runner.Run(context.Background(), t.TempDir(), "prompt", longPrompt); err != nil {
		t.Fatal(err)
	}
	argsBytes, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if len(argsBytes) > 4096 {
		t.Fatalf("CLI arguments remain too large: %d bytes", len(argsBytes))
	}
	promptBytes, err := os.ReadFile(promptCapture)
	if err != nil {
		t.Fatal(err)
	}
	if string(promptBytes) != longPrompt {
		t.Fatalf("large externalized prompt mismatch: got %d bytes want %d", len(promptBytes), len(longPrompt))
	}
}

func TestExternalizedPromptUsesSingleLineAbsolutePortablePath(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "Project With Spaces", "資料")
	cliPrompt, toolPath, cleanup, err := externalizePrompt(workdir, "hello")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if strings.ContainsAny(cliPrompt, "\r\n") {
		t.Fatalf("prompt bridge must be single-line: %q", cliPrompt)
	}
	extracted, err := promptPathFromBridge(cliPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if extracted != toolPath {
		t.Fatalf("path mismatch: got %q want %q", extracted, toolPath)
	}
	if strings.ContainsAny(toolPath, "\\\"\r\n") {
		t.Fatalf("tool path is not portable: %q", toolPath)
	}
	if !filepath.IsAbs(filepath.FromSlash(toolPath)) {
		t.Fatalf("tool path is not absolute: %q", toolPath)
	}
	b, err := os.ReadFile(filepath.FromSlash(toolPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("prompt mismatch: %q", string(b))
	}
}

func TestExecRunnerRejectsClarificationResponse(t *testing.T) {
	runner := ExecRunner{Config: config.OpenWikiConfig{
		Command:        os.Args[0],
		Args:           []string{"-test.run=TestOpenWikiHelperProcess", "--", "code"},
		TimeoutMinutes: 1,
		Environment: map[string]string{
			"WIKIFORGE_HELPER_PROCESS":      "1",
			"WIKIFORGE_CLARIFICATION_TEST":  "1",
			"WIKIFORGE_CAPTURE_PROMPT_PATH": filepath.Join(t.TempDir(), "prompt.txt"),
		},
	}}
	output, err := runner.Run(context.Background(), t.TempDir(), "prompt", "phase prompt")
	if err == nil {
		t.Fatalf("clarification response must fail; output=%q", output)
	}
	if !strings.Contains(err.Error(), "clarification") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecRunnerCheck(t *testing.T) {
	capture := filepath.Join(t.TempDir(), "args.txt")
	runner := ExecRunner{Config: config.OpenWikiConfig{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestOpenWikiHelperProcess", "--", "code"},
		Environment: map[string]string{
			"WIKIFORGE_HELPER_PROCESS": "1",
			"WIKIFORGE_CAPTURE_PATH":   capture,
		},
	}}
	if err := runner.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "--help") {
		t.Fatalf("help flag missing: %q", string(b))
	}
}

func TestExecRunnerStreamsOutputAndHeartbeat(t *testing.T) {
	var out bytes.Buffer
	runner := ExecRunner{
		Config: config.OpenWikiConfig{
			Command:        os.Args[0],
			Args:           []string{"-test.run=TestOpenWikiHelperProcess", "--", "code"},
			TimeoutMinutes: 1,
			Environment: map[string]string{
				"WIKIFORGE_HELPER_PROCESS": "1",
				"WIKIFORGE_STREAM_TEST":    "1",
			},
		},
		Out:               &out,
		LiveOutput:        true,
		HeartbeatInterval: 10 * time.Millisecond,
	}
	ctx := WithRunLabel(context.Background(), "sentinel/A00")
	if _, err := runner.Run(ctx, t.TempDir(), "prompt", "phase prompt"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"sentinel/A00", "OpenWiki process started", "repository scan progress", "provider progress", "still running", "OpenWiki process completed"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in streamed output:\n%s", expected, text)
		}
	}
}

func TestOpenWikiHelperProcess(t *testing.T) {
	if os.Getenv("WIKIFORGE_HELPER_PROCESS") != "1" {
		return
	}
	if os.Getenv("WIKIFORGE_STREAM_TEST") == "1" {
		fmt.Println("repository scan progress")
		fmt.Fprintln(os.Stderr, "provider progress")
		time.Sleep(60 * time.Millisecond)
		os.Exit(0)
	}
	if os.Getenv("WIKIFORGE_CLARIFICATION_TEST") == "1" {
		fmt.Println("I do not see a file path. Could you clarify what you would like me to do?")
		os.Exit(0)
	}
	path := os.Getenv("WIKIFORGE_CAPTURE_PATH")
	if path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(os.Args, "\n")), 0o644)
	}
	if promptCapture := os.Getenv("WIKIFORGE_CAPTURE_PROMPT_PATH"); promptCapture != "" {
		for _, arg := range os.Args {
			toolPath, err := promptPathFromBridge(arg)
			if err != nil {
				continue
			}
			if strings.ContainsAny(toolPath, "\"\r\n") {
				os.Exit(23)
			}
			b, err := os.ReadFile(filepath.FromSlash(toolPath))
			if err == nil {
				_ = os.WriteFile(promptCapture, b, 0o644)
			}
			break
		}
	}
	os.Exit(0)
}

func TestLooksLikeClarificationResponse(t *testing.T) {
	for _, output := range []string{
		`I see you mentioned a WikiForge task specification. Could you clarify what you'd like me to do?`,
		`Do you have a file path for that WikiForge task specification?`,
		`Please provide the absolute path and let me know what you need.`,
	} {
		if !looksLikeClarificationResponse(output) {
			t.Fatalf("expected clarification response: %q", output)
		}
	}
	if looksLikeClarificationResponse("All four specialized catalog pages are now complete.") {
		t.Fatal("successful completion must not be classified as clarification")
	}
}

func TestIsNonRetryableError(t *testing.T) {
	for _, message := range []string{
		`OpenWiki failed: path must be absolute: "C:\\repo\\prompt.md"`,
		"The command line is too long.",
		"externalize OpenWiki prompt: portable absolute path: invalid",
	} {
		if !IsNonRetryableError(fmt.Errorf("%s", message)) {
			t.Fatalf("expected non-retryable: %s", message)
		}
	}
	if IsNonRetryableError(fmt.Errorf("provider returned HTTP 503")) {
		t.Fatal("transient provider failure should remain retryable")
	}
	if IsNonRetryableError(fmt.Errorf("OpenWiki returned a clarification instead of executing the WikiForge task")) {
		t.Fatal("clarification responses should be retryable")
	}
}
