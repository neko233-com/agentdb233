package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/neko233-com/agentdb233/internal/contextpack"
	"github.com/neko233-com/agentdb233/internal/gitx"
	"github.com/neko233-com/agentdb233/internal/indexer"
	"github.com/neko233-com/agentdb233/internal/store"
)

type Server struct {
	store *store.Store
}

func Run(dataDir string, in io.Reader, out io.Writer) error {
	st := store.New(dataDir)
	if err := st.Init(); err != nil {
		return err
	}
	s := &Server{store: st}
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		resp := s.handle(req)
		b, _ := json.Marshal(resp)
		_, _ = fmt.Fprintln(out, string(b))
	}
	return scanner.Err()
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func (s *Server) handle(req request) response {
	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]string{"name": "agentdb233", "version": "dev"},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "tools/list":
		res.Result = map[string]any{"tools": tools()}
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			res.Error = map[string]any{"code": -32000, "message": err.Error()}
		} else {
			res.Result = result
		}
	default:
		res.Error = map[string]any{"code": -32601, "message": "method not found"}
	}
	return res
}

func (s *Server) callTool(raw json.RawMessage) (any, error) {
	var req struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	switch req.Name {
	case "build_index":
		project, repo := strArg(req.Arguments, "project"), strArg(req.Arguments, "repo")
		idx, err := indexer.BuildWithOptions(indexer.BuildOptions{Project: project, Repo: repo, TrackedOnly: boolArg(req.Arguments, "tracked_only")})
		if err != nil {
			return nil, err
		}
		if err := indexer.Save(s.store.IndexPath(project), idx); err != nil {
			return nil, err
		}
		return textResult(fmt.Sprintf("indexed_files=%d reused_files=%d skipped_files=%d chunks=%d", idx.Stats.IndexedFiles, idx.Stats.ReusedFiles, idx.Stats.SkippedFiles, idx.Stats.Chunks)), nil
	case "search_context":
		project, query := strArg(req.Arguments, "project"), strArg(req.Arguments, "query")
		if project == "" {
			project = "default"
		}
		idx, err := indexer.Load(s.store.IndexPath(project))
		if err != nil {
			return nil, err
		}
		results := indexer.Search(idx, query, intArg(req.Arguments, "limit"))
		knowledge, _ := s.store.ListKnowledge(project, query)
		pack := contextpack.Build(project, query, intArg(req.Arguments, "budget"), results, knowledge)
		return textResult(pack.Text), nil
	case "git_refs":
		refs, err := gitx.ListRefs(strArg(req.Arguments, "repo"))
		if err != nil {
			return nil, err
		}
		b, _ := json.MarshalIndent(refs, "", "  ")
		return textResult(string(b)), nil
	default:
		return nil, fmt.Errorf("unknown tool %q", req.Name)
	}
}

func tools() []map[string]any {
	return []map[string]any{
		{"name": "build_index", "description": "Build shared agentdb233 index for a repo", "inputSchema": schema(map[string]any{"project": "string", "repo": "string", "tracked_only": "boolean"})},
		{"name": "search_context", "description": "Return compact shared context pack for all AI agents", "inputSchema": schema(map[string]any{"project": "string", "query": "string", "limit": "number", "budget": "number"})},
		{"name": "git_refs", "description": "List git branches and tags", "inputSchema": schema(map[string]any{"repo": "string"})},
	}
}

func schema(props map[string]any) map[string]any {
	properties := map[string]any{}
	for name, typ := range props {
		properties[name] = map[string]any{"type": typ}
	}
	return map[string]any{"type": "object", "properties": properties}
}

func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]string{{"type": "text", "text": text}}}
}

func strArg(args map[string]any, name string) string {
	if v, ok := args[name].(string); ok {
		return v
	}
	return ""
}

func boolArg(args map[string]any, name string) bool {
	if v, ok := args[name].(bool); ok {
		return v
	}
	return false
}

func intArg(args map[string]any, name string) int {
	switch v := args[name].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func RunStdio(dataDir string) error {
	return Run(dataDir, os.Stdin, os.Stdout)
}
