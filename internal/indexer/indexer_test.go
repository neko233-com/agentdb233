package indexer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageForPath(t *testing.T) {
	cases := map[string]string{
		"a.cs":   "csharp",
		"a.go":   "go",
		"a.ts":   "typescript",
		"a.tsx":  "typescript",
		"a.html": "html",
		"a.yml":  "config",
		"a.sh":   "script",
		"a.rs":   "rust",
		"a.cpp":  "cpp",
		"a.java": "java",
		"a.kt":   "kotlin",
		"a.py":   "python",
		"a.md":   "docs",
		"a.bin":  "",
	}
	for path, want := range cases {
		if got := LanguageForPath(path); got != want {
			t.Fatalf("LanguageForPath(%q)=%q want %q", path, got, want)
		}
	}
}

func TestBuildAndSearch(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "main.go"), "package main\n\nfunc SearchBrain() string {\n\treturn \"agent context\"\n}\n")
	write(t, filepath.Join(dir, "tool.ts"), "export function buildIndex() {\n  return 'typescript';\n}\n")
	write(t, filepath.Join(dir, "Agent.cs"), "public class AgentBrain {\n  public void Recall() {}\n}\n")
	write(t, filepath.Join(dir, "guide.html"), "<html><body>\n<h1>Agent retrieval guide</h1>\n<p>html docs are agent output</p>\n</body></html>\n")
	write(t, filepath.Join(dir, "config.yaml"), "retrieval:\n  mode: compact\n")
	write(t, filepath.Join(dir, "deploy.sh"), "function deploy_agentdb() {\n  echo deploy\n}\n")
	write(t, filepath.Join(dir, "App.java"), "public class App {\n  void run() {}\n}\n")
	write(t, filepath.Join(dir, "Main.kt"), "class Main {\n  fun run() {}\n}\n")
	write(t, filepath.Join(dir, "brain.py"), "class Memory:\n    def recall(self):\n        return 'python'\n")
	write(t, filepath.Join(dir, "node_modules", "skip.ts"), "export const noise = true\n")
	write(t, filepath.Join(dir, ".agentdb233-server-smoke", "run", "server-state.json"), `{"secret":"runtime"}`)
	write(t, filepath.Join(dir, "bundle.min.js"), "function minifiedNoise(){return true}\n")
	write(t, filepath.Join(dir, "image.png"), "png\x00binary\n")

	idx, err := Build("demo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Chunks) < 6 {
		t.Fatalf("chunks=%d want >= 6", len(idx.Chunks))
	}
	if idx.Stats.IndexedFiles < 9 {
		t.Fatalf("indexed files=%d want >= 9", idx.Stats.IndexedFiles)
	}
	if idx.Stats.SkippedFiles < 2 {
		t.Fatalf("skipped files=%d want >= 2", idx.Stats.SkippedFiles)
	}
	for _, ch := range idx.Chunks {
		if ch.Path == "node_modules/skip.ts" {
			t.Fatal("node_modules was indexed")
		}
		if ch.Path == "bundle.min.js" {
			t.Fatal("minified file was indexed")
		}
		if ch.Path == "image.png" {
			t.Fatal("binary file was indexed")
		}
		if ch.Path == ".agentdb233-server-smoke/run/server-state.json" {
			t.Fatal("agentdb233 data dir was indexed")
		}
	}
	results := Search(idx, "SearchBrain context", 5)
	if len(results) == 0 {
		t.Fatal("expected search result")
	}
	if results[0].Language != "go" {
		t.Fatalf("top language=%s want go", results[0].Language)
	}
	htmlResults := Search(idx, "html agent output", 5)
	if len(htmlResults) == 0 || htmlResults[0].Language != "html" {
		t.Fatalf("html search top=%v", htmlResults)
	}
}

func TestSupportedLanguagesMetadata(t *testing.T) {
	specs := SupportedLanguages()
	if len(specs) < 20 {
		t.Fatalf("languages=%d want >= 20", len(specs))
	}
	if len(DefaultSkipDirs()) == 0 || len(DefaultSkipFiles()) == 0 {
		t.Fatal("skip metadata empty")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
