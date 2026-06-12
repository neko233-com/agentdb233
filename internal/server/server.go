package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neko233-com/agentdb233/internal/contextpack"
	"github.com/neko233-com/agentdb233/internal/gitx"
	"github.com/neko233-com/agentdb233/internal/indexer"
	"github.com/neko233-com/agentdb233/internal/model"
	"github.com/neko233-com/agentdb233/internal/store"
	"github.com/neko233-com/agentdb233/internal/version"
)

type Server struct {
	store *store.Store
}

func New(dataDir string) *Server {
	st := store.New(dataDir)
	_ = st.Init()
	return &Server{store: st}
}

func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.health)
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/knowledge", s.listKnowledge)
	mux.HandleFunc("POST /api/knowledge", s.addKnowledge)
	mux.HandleFunc("GET /api/skills", s.listSkills)
	mux.HandleFunc("POST /api/skills", s.upsertSkill)
	mux.HandleFunc("GET /api/mcp", s.listMCP)
	mux.HandleFunc("POST /api/mcp", s.upsertMCP)
	mux.HandleFunc("GET /api/git/refs", s.gitRefs)
	mux.HandleFunc("GET /api/git/commits", s.gitCommits)
	mux.HandleFunc("GET /api/git/status", s.gitStatus)
	mux.HandleFunc("GET /api/index/languages", s.indexLanguages)
	mux.HandleFunc("POST /api/index/build", s.buildIndex)
	mux.HandleFunc("GET /api/index/search", s.searchIndex)
	mux.HandleFunc("GET /api/context/pack", s.contextPack)
	mux.HandleFunc("POST /api/norms/import", s.importNorms)
	mux.HandleFunc("/", s.index)
	return withCORS(mux)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) status(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "agentdb233-server",
		"version": version.String("agentdb233-server"),
		"data":    s.store.DataDir(),
		"features": []string{
			"knowledge-base",
			"git-branches-tags-commits",
			"skill-registry",
			"mcp-registry",
			"code-index-search",
			"code-index-stats",
			"context-pack",
			"norms-import",
			"asset-industry-metadata",
		},
	})
}

func (s *Server) listKnowledge(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListKnowledge(r.URL.Query().Get("project"), r.URL.Query().Get("q"))
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) addKnowledge(w http.ResponseWriter, r *http.Request) {
	var entry model.KnowledgeEntry
	if err := readJSON(r, &entry); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.store.AddKnowledge(entry)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listSkills(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListSkills()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) upsertSkill(w http.ResponseWriter, r *http.Request) {
	var item model.Skill
	if err := readJSON(r, &item); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.store.UpsertSkill(item)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listMCP(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.ListMCP()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) upsertMCP(w http.ResponseWriter, r *http.Request) {
	var item model.MCPEndpoint
	if err := readJSON(r, &item); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.store.UpsertMCP(item)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) gitRefs(w http.ResponseWriter, r *http.Request) {
	refs, err := gitx.ListRefs(r.URL.Query().Get("repo"))
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, refs)
}

func (s *Server) gitCommits(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := gitx.ListCommits(r.URL.Query().Get("repo"), r.URL.Query().Get("ref"), limit)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) gitStatus(w http.ResponseWriter, r *http.Request) {
	st, err := gitx.WorktreeStatus(r.URL.Query().Get("repo"))
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) indexLanguages(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"languages":  indexer.SupportedLanguages(),
		"skip_dirs":  indexer.DefaultSkipDirs(),
		"skip_files": indexer.DefaultSkipFiles(),
		"max_bytes":  indexer.MaxFileBytes,
	})
}

func (s *Server) buildIndex(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project     string `json:"project"`
		Repo        string `json:"repo"`
		Ref         string `json:"ref"`
		TrackedOnly bool   `json:"tracked_only"`
		Incremental bool   `json:"incremental"`
	}
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	var previous *indexer.Index
	if req.Incremental {
		if old, err := indexer.Load(s.indexPath(req.Project)); err == nil {
			previous = &old
		}
	}
	idx, err := indexer.BuildWithOptions(indexer.BuildOptions{Project: req.Project, Repo: req.Repo, Ref: req.Ref, TrackedOnly: req.TrackedOnly, Previous: previous})
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := indexer.Save(s.indexPath(idx.Project), idx); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"project": idx.Project,
		"repo":    idx.Repo,
		"stats":   idx.Stats,
		"files":   idx.Files,
	})
}

func (s *Server) searchIndex(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	idx, err := indexer.Load(s.indexPath(project))
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, indexer.Search(idx, r.URL.Query().Get("q"), limit))
}

func (s *Server) contextPack(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		project = "default"
	}
	query := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	budget, _ := strconv.Atoi(r.URL.Query().Get("budget"))
	idx, err := indexer.Load(s.indexPath(project))
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	results := indexer.Search(idx, query, limit)
	knowledge, err := s.store.ListKnowledge(project, query)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(knowledge) > 10 {
		knowledge = knowledge[:10]
	}
	writeJSON(w, http.StatusOK, contextpack.Build(project, query, budget, results, knowledge))
}

func (s *Server) importNorms(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Project string `json:"project"`
		Repo    string `json:"repo"`
	}
	if err := readJSON(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	project := req.Project
	if project == "" {
		project = "default"
	}
	repo, err := filepath.Abs(req.Repo)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	names := []string{"AGENTS.md", "CLAUDE.md", ".clinerules", ".cursorrules", "README.md", "CONTRIBUTING.md"}
	var imported []model.KnowledgeEntry
	for _, name := range names {
		path := filepath.Join(repo, name)
		b, err := os.ReadFile(path)
		if err != nil || len(strings.TrimSpace(string(b))) == 0 {
			continue
		}
		entry, err := s.store.AddKnowledge(model.KnowledgeEntry{
			Project: project,
			Kind:    "norm",
			Title:   name,
			Body:    string(b),
			Tags:    []string{"norms", "shared-ai-context"},
			Git:     model.GitRef{Repo: repo},
		})
		if err != nil {
			errJSON(w, http.StatusInternalServerError, err.Error())
			return
		}
		imported = append(imported, entry)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": project, "imported": imported})
}

func (s *Server) indexPath(project string) string {
	return s.store.IndexPath(project)
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>agentdb233</title><style>body{font-family:system-ui;margin:40px;max-width:900px}code{background:#eee;padding:2px 5px;border-radius:4px}</style><h1>agentdb233</h1><p>Agent external brain volume running.</p><p>API: <code>/api/status</code>, <code>/api/knowledge</code>, <code>/api/index/build</code>, <code>/api/index/search</code>, <code>/api/context/pack</code>, <code>/api/git/refs</code>, <code>/api/skills</code>, <code>/api/mcp</code></p>`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("access-control-allow-origin", "*")
		w.Header().Set("access-control-allow-methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("access-control-allow-headers", "content-type,authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
