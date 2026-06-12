package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPToolsListAndBuildSearch(t *testing.T) {
	data := t.TempDir()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "brain.go"), []byte("package demo\nfunc SharedBrain() string { return \"context\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"build_index","arguments":{"project":"demo","repo":` + quoteJSON(repo) + `}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_context","arguments":{"project":"demo","query":"SharedBrain","budget":2000}}}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := Run(data, strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines=%d output=%s", len(lines), out.String())
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &last); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(last)
	if !bytes.Contains(b, []byte("SharedBrain")) {
		t.Fatalf("missing search result: %s", string(b))
	}
}

func quoteJSON(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
