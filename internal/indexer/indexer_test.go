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
	write(t, filepath.Join(dir, "App.java"), "public class App {\n  void run() {}\n}\n")
	write(t, filepath.Join(dir, "Main.kt"), "class Main {\n  fun run() {}\n}\n")
	write(t, filepath.Join(dir, "brain.py"), "class Memory:\n    def recall(self):\n        return 'python'\n")
	write(t, filepath.Join(dir, "node_modules", "skip.ts"), "export const noise = true\n")

	idx, err := Build("demo", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Chunks) < 6 {
		t.Fatalf("chunks=%d want >= 6", len(idx.Chunks))
	}
	for _, ch := range idx.Chunks {
		if ch.Path == "node_modules/skip.ts" {
			t.Fatal("node_modules was indexed")
		}
	}
	results := Search(idx, "SearchBrain context", 5)
	if len(results) == 0 {
		t.Fatal("expected search result")
	}
	if results[0].Language != "go" {
		t.Fatalf("top language=%s want go", results[0].Language)
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
