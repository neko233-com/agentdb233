package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestIndexBuildSearchAPI(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "brain.py"), []byte("def recall_context():\n    return 'fast retrieval'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(t.TempDir()).Router())
	defer srv.Close()

	body := []byte(`{"project":"demo","repo":` + quote(repo) + `}`)
	resp, err := http.Post(srv.URL+"/api/index/build", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("build status=%d", resp.StatusCode)
	}
	var build map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&build); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if build["stats"] == nil || build["files"] == nil {
		t.Fatalf("build response missing stats/files: %+v", build)
	}

	resp, err = http.Get(srv.URL + "/api/index/search?project=demo&q=fast%20retrieval")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search status=%d", resp.StatusCode)
	}
	var results []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0]["language"] != "python" {
		t.Fatalf("language=%v want python", results[0]["language"])
	}

	resp, err = http.Get(srv.URL + "/api/index/languages")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("languages status=%d", resp.StatusCode)
	}
	var meta map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta["languages"] == nil || meta["skip_dirs"] == nil || meta["max_bytes"] == nil {
		t.Fatalf("languages meta=%+v", meta)
	}

	resp, err = http.Get(srv.URL + "/api/context/pack?project=demo&q=fast%20retrieval&budget=2000")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("context pack status=%d", resp.StatusCode)
	}
	var pack map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&pack); err != nil {
		t.Fatal(err)
	}
	text, _ := pack["text"].(string)
	if !bytes.Contains([]byte(text), []byte("fast retrieval")) {
		t.Fatalf("pack text=%q", text)
	}
}

func TestImportNormsAPI(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("Use shared agentdb233 norms.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(t.TempDir()).Router())
	defer srv.Close()
	body := []byte(`{"project":"demo","repo":` + quote(repo) + `}`)
	resp, err := http.Post(srv.URL+"/api/norms/import", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("import status=%d", resp.StatusCode)
	}
	resp, err = http.Get(srv.URL + "/api/knowledge?project=demo&q=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var entries []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected imported norm")
	}
}

func quote(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
}
