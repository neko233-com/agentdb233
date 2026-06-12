package indexer

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

const MaxFileBytes = 512 * 1024

type Chunk struct {
	ID        string   `json:"id"`
	Project   string   `json:"project"`
	Repo      string   `json:"repo"`
	Path      string   `json:"path"`
	Language  string   `json:"language"`
	Symbol    string   `json:"symbol,omitempty"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
	Text      string   `json:"text"`
	Tokens    []string `json:"tokens,omitempty"`
}

type Index struct {
	Project string  `json:"project"`
	Repo    string  `json:"repo"`
	Chunks  []Chunk `json:"chunks"`
}

type Result struct {
	Chunk
	Score int `json:"score"`
}

func Build(project, repo string) (Index, error) {
	if strings.TrimSpace(project) == "" {
		project = "default"
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return Index{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Index{}, err
	}
	if !info.IsDir() {
		return Index{}, errors.New("repo must be a directory")
	}
	var chunks []Chunk
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != abs {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipFile(d.Name()) {
			return nil
		}
		lang := LanguageForPath(path)
		if lang == "" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxFileBytes {
			return nil
		}
		rel, err := filepath.Rel(abs, path)
		if err != nil {
			return err
		}
		fileChunks, err := ChunkFile(project, abs, filepath.ToSlash(rel), lang, path)
		if err != nil {
			return err
		}
		chunks = append(chunks, fileChunks...)
		return nil
	})
	if err != nil {
		return Index{}, err
	}
	sort.Slice(chunks, func(i, j int) bool {
		if chunks[i].Path == chunks[j].Path {
			return chunks[i].StartLine < chunks[j].StartLine
		}
		return chunks[i].Path < chunks[j].Path
	})
	for i := range chunks {
		chunks[i].ID = project + ":" + chunks[i].Path + ":" + itoa(chunks[i].StartLine)
	}
	return Index{Project: project, Repo: abs, Chunks: chunks}, nil
}

func Save(path string, idx Index) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func Load(path string) (Index, error) {
	var idx Index
	b, err := os.ReadFile(path)
	if err != nil {
		return idx, err
	}
	return idx, json.Unmarshal(b, &idx)
}

func Search(idx Index, query string, limit int) []Result {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	terms := Tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	var out []Result
	for _, ch := range idx.Chunks {
		score := scoreChunk(ch, terms)
		if score > 0 {
			out = append(out, Result{Chunk: ch, Score: score})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Path < out[j].Path
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func ChunkFile(project, repo, rel, lang, path string) ([]Chunk, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
	start := 1
	symbol := ""
	for i := 0; i < len(lines); i++ {
		if next := DetectSymbol(lang, lines[i]); next != "" {
			if i+1 > start {
				chunks = append(chunks, makeChunk(project, repo, rel, lang, symbol, start, i, lines[start-1:i]))
			}
			start = i + 1
			symbol = next
		}
		if i-start+1 >= 80 {
			chunks = append(chunks, makeChunk(project, repo, rel, lang, symbol, start, i+1, lines[start-1:i+1]))
			start = i + 2
		}
	}
	if start <= len(lines) {
		chunks = append(chunks, makeChunk(project, repo, rel, lang, symbol, start, len(lines), lines[start-1:]))
	}
	return chunks, nil
}

func LanguageForPath(path string) string {
	base := strings.ToLower(filepath.Base(path))
	switch base {
	case "dockerfile", "makefile", "justfile", "gemfile", "rakefile":
		return strings.TrimSuffix(base, "file")
	case "requirements.txt", "pyproject.toml", "package.json", "tsconfig.json", "go.mod", "go.sum", "pom.xml", "build.gradle", "settings.gradle", "gradle.properties":
		return "manifest"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cs":
		return "csharp"
	case ".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	case ".astro":
		return "astro"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".lua":
		return "lua"
	case ".md", ".mdx", ".markdown", ".txt", ".rst", ".adoc":
		return "docs"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass", ".less":
		return "style"
	case ".json", ".jsonc", ".yaml", ".yml", ".toml", ".ini", ".env", ".properties", ".xml":
		return "config"
	case ".sql", ".graphql", ".gql":
		return "query"
	case ".sh", ".bash", ".zsh", ".fish", ".ps1", ".psm1", ".bat", ".cmd":
		return "script"
	case ".csv", ".tsv":
		return "data"
	default:
		return ""
	}
}

func DetectSymbol(lang, line string) string {
	s := strings.TrimSpace(line)
	switch lang {
	case "go":
		if strings.HasPrefix(s, "func ") || strings.HasPrefix(s, "type ") {
			return compactSymbol(s)
		}
	case "python":
		if strings.HasPrefix(s, "def ") || strings.HasPrefix(s, "class ") {
			return compactSymbol(s)
		}
	case "typescript", "javascript", "vue", "svelte", "astro":
		if strings.HasPrefix(s, "export ") || strings.HasPrefix(s, "function ") || strings.Contains(s, "=>") || strings.Contains(s, " class ") {
			return compactSymbol(s)
		}
	case "java", "kotlin", "csharp", "cpp", "rust", "swift", "php":
		if strings.Contains(s, " class ") || strings.HasPrefix(s, "class ") || strings.Contains(s, " interface ") || strings.Contains(s, " fun ") || strings.HasPrefix(s, "fun ") || strings.Contains(s, " void ") {
			return compactSymbol(s)
		}
		if strings.HasPrefix(s, "func ") || strings.HasPrefix(s, "fn ") || strings.HasPrefix(s, "struct ") || strings.HasPrefix(s, "enum ") {
			return compactSymbol(s)
		}
	case "ruby":
		if strings.HasPrefix(s, "def ") || strings.HasPrefix(s, "class ") || strings.HasPrefix(s, "module ") {
			return compactSymbol(s)
		}
	case "lua":
		if strings.HasPrefix(s, "function ") || strings.Contains(s, "= function") {
			return compactSymbol(s)
		}
	case "docs":
		if strings.HasPrefix(s, "#") {
			return compactSymbol(s)
		}
	case "html":
		lower := strings.ToLower(s)
		if strings.HasPrefix(lower, "<title") || strings.HasPrefix(lower, "<h1") || strings.HasPrefix(lower, "<h2") || strings.HasPrefix(lower, "<h3") || strings.HasPrefix(lower, "<section") || strings.HasPrefix(lower, "<article") {
			return compactSymbol(stripHTML(compactSymbol(s)))
		}
	case "style":
		if strings.HasSuffix(s, "{") {
			return compactSymbol(s)
		}
	case "config", "manifest":
		if strings.Contains(s, ":") || strings.Contains(s, "=") || strings.HasPrefix(s, "[") {
			return compactSymbol(s)
		}
	case "script":
		if strings.HasPrefix(s, "function ") || strings.HasPrefix(s, "param(") || strings.HasPrefix(s, "Param(") {
			return compactSymbol(s)
		}
	case "query":
		lower := strings.ToLower(s)
		if strings.HasPrefix(lower, "select ") || strings.HasPrefix(lower, "create ") || strings.HasPrefix(lower, "mutation ") || strings.HasPrefix(lower, "query ") {
			return compactSymbol(s)
		}
	}
	return ""
}

func Tokenize(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	}) {
		part = strings.Trim(part, "_-")
		if len(part) < 2 || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func scoreChunk(ch Chunk, terms []string) int {
	text := strings.ToLower(ch.Path + "\n" + ch.Language + "\n" + ch.Symbol + "\n" + ch.Text)
	score := 0
	for _, term := range terms {
		if strings.Contains(strings.ToLower(ch.Path), term) {
			score += 8
		}
		if strings.Contains(strings.ToLower(ch.Symbol), term) {
			score += 6
		}
		score += strings.Count(text, term)
	}
	return score
}

func makeChunk(project, repo, rel, lang, symbol string, start, end int, lines []string) Chunk {
	text := strings.Join(lines, "\n")
	return Chunk{
		Project:   project,
		Repo:      repo,
		Path:      rel,
		Language:  lang,
		Symbol:    symbol,
		StartLine: start,
		EndLine:   end,
		Text:      text,
		Tokens:    Tokenize(rel + "\n" + symbol + "\n" + text),
	}
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var lines []string
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func shouldSkipDir(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "bin", "obj", "dist", "build", "target", ".next", ".vite", ".turbo", ".cache", "coverage", "__pycache__", ".venv", "venv", ".gradle", ".idea", ".vs":
		return true
	default:
		return false
	}
}

func shouldSkipFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".min.js") || strings.HasSuffix(lower, ".min.css") || strings.HasSuffix(lower, ".map") {
		return true
	}
	switch lower {
	case "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb", "go.sum", "poetry.lock", "cargo.lock", "gradle.lockfile":
		return true
	default:
		return false
	}
}

func compactSymbol(s string) string {
	if len(s) > 160 {
		return s[:160]
	}
	return s
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return s
	}
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
