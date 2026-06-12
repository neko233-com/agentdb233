package indexer

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	Project string      `json:"project"`
	Repo    string      `json:"repo"`
	Ref     string      `json:"ref,omitempty"`
	Stats   BuildStats  `json:"stats"`
	Chunks  []Chunk     `json:"chunks"`
	Files   []FileEntry `json:"files,omitempty"`
}

type Result struct {
	Chunk
	Score int `json:"score"`
}

type BuildStats struct {
	IndexedFiles int `json:"indexed_files"`
	ReusedFiles  int `json:"reused_files"`
	SkippedFiles int `json:"skipped_files"`
	Chunks       int `json:"chunks"`
}

type FileEntry struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Chunks   int    `json:"chunks"`
	Hash     string `json:"hash,omitempty"`
	Size     int64  `json:"size,omitempty"`
	ModTime  string `json:"mod_time,omitempty"`
}

type LanguageSpec struct {
	Language   string   `json:"language"`
	Extensions []string `json:"extensions"`
	Files      []string `json:"files,omitempty"`
}

type BuildOptions struct {
	Project     string
	Repo        string
	Ref         string
	TrackedOnly bool
	Previous    *Index
}

func Build(project, repo string) (Index, error) {
	return BuildWithOptions(BuildOptions{Project: project, Repo: repo})
}

func BuildWithOptions(opts BuildOptions) (Index, error) {
	project := opts.Project
	if strings.TrimSpace(project) == "" {
		project = "default"
	}
	abs, err := filepath.Abs(opts.Repo)
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
	if strings.TrimSpace(opts.Ref) != "" {
		return buildGitRef(project, abs, opts.Ref)
	}
	candidates, skipped, err := candidateFiles(abs, opts.TrackedOnly)
	if err != nil {
		return Index{}, err
	}
	var chunks []Chunk
	var files []FileEntry
	stats := BuildStats{SkippedFiles: skipped}
	prev := previousByPath(opts.Previous)
	for _, rel := range candidates {
		path := filepath.Join(abs, filepath.FromSlash(rel))
		lang := LanguageForPath(path)
		if lang == "" || shouldSkipFile(filepath.Base(path)) {
			stats.SkippedFiles++
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			stats.SkippedFiles++
			continue
		}
		if info.Size() > MaxFileBytes {
			stats.SkippedFiles++
			continue
		}
		binary, err := isBinaryFile(path)
		if err != nil {
			return Index{}, err
		}
		if binary {
			stats.SkippedFiles++
			continue
		}
		hash, err := fileSHA256(path)
		if err != nil {
			return Index{}, err
		}
		if old, ok := prev[rel]; ok && old.Hash == hash {
			reused := chunksForFile(opts.Previous, rel)
			chunks = append(chunks, reused...)
			files = append(files, old)
			stats.ReusedFiles++
			continue
		}
		fileChunks, err := ChunkFile(project, abs, rel, lang, path)
		if err != nil {
			return Index{}, err
		}
		stats.IndexedFiles++
		files = append(files, FileEntry{Path: rel, Language: lang, Chunks: len(fileChunks), Hash: hash, Size: info.Size(), ModTime: info.ModTime().UTC().Format("2006-01-02T15:04:05Z")})
		chunks = append(chunks, fileChunks...)
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
	stats.Chunks = len(chunks)
	return Index{Project: project, Repo: abs, Stats: stats, Chunks: chunks, Files: files}, nil
}

func buildGitRef(project, repo, ref string) (Index, error) {
	files, err := gitLines(repo, "ls-tree", "-r", "--name-only", ref)
	if err != nil {
		return Index{}, err
	}
	idx := Index{Project: project, Repo: repo, Ref: ref}
	for _, rel := range files {
		rel = filepath.ToSlash(rel)
		if shouldSkipFile(filepath.Base(rel)) {
			idx.Stats.SkippedFiles++
			continue
		}
		lang := LanguageForPath(rel)
		if lang == "" {
			idx.Stats.SkippedFiles++
			continue
		}
		content, err := gitBytes(repo, "show", ref+":"+rel)
		if err != nil {
			idx.Stats.SkippedFiles++
			continue
		}
		if int64(len(content)) > MaxFileBytes || bytes.IndexByte(content, 0) >= 0 {
			idx.Stats.SkippedFiles++
			continue
		}
		sum := sha256.Sum256(content)
		chunks := ChunkText(project, repo, rel, lang, string(content))
		idx.Files = append(idx.Files, FileEntry{Path: rel, Language: lang, Chunks: len(chunks), Hash: hex.EncodeToString(sum[:]), Size: int64(len(content))})
		idx.Chunks = append(idx.Chunks, chunks...)
		idx.Stats.IndexedFiles++
	}
	sort.Slice(idx.Chunks, func(i, j int) bool {
		if idx.Chunks[i].Path == idx.Chunks[j].Path {
			return idx.Chunks[i].StartLine < idx.Chunks[j].StartLine
		}
		return idx.Chunks[i].Path < idx.Chunks[j].Path
	})
	for i := range idx.Chunks {
		idx.Chunks[i].ID = project + ":" + idx.Chunks[i].Path + ":" + itoa(idx.Chunks[i].StartLine)
	}
	idx.Stats.Chunks = len(idx.Chunks)
	return idx, nil
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
	return chunkLines(project, repo, rel, lang, lines), nil
}

func ChunkText(project, repo, rel, lang, text string) []Chunk {
	return chunkLines(project, repo, rel, lang, strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n"))
}

func chunkLines(project, repo, rel, lang string, lines []string) []Chunk {
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
	return chunks
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

func SupportedLanguages() []LanguageSpec {
	return []LanguageSpec{
		{Language: "csharp", Extensions: []string{".cs"}},
		{Language: "cpp", Extensions: []string{".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx"}},
		{Language: "go", Extensions: []string{".go"}},
		{Language: "typescript", Extensions: []string{".ts", ".tsx"}},
		{Language: "javascript", Extensions: []string{".js", ".jsx", ".mjs", ".cjs"}},
		{Language: "vue", Extensions: []string{".vue"}},
		{Language: "svelte", Extensions: []string{".svelte"}},
		{Language: "astro", Extensions: []string{".astro"}},
		{Language: "java", Extensions: []string{".java"}},
		{Language: "kotlin", Extensions: []string{".kt", ".kts"}},
		{Language: "python", Extensions: []string{".py"}},
		{Language: "rust", Extensions: []string{".rs"}},
		{Language: "ruby", Extensions: []string{".rb"}},
		{Language: "php", Extensions: []string{".php"}},
		{Language: "swift", Extensions: []string{".swift"}},
		{Language: "lua", Extensions: []string{".lua"}},
		{Language: "docs", Extensions: []string{".md", ".mdx", ".markdown", ".txt", ".rst", ".adoc"}},
		{Language: "html", Extensions: []string{".html", ".htm"}},
		{Language: "style", Extensions: []string{".css", ".scss", ".sass", ".less"}},
		{Language: "config", Extensions: []string{".json", ".jsonc", ".yaml", ".yml", ".toml", ".ini", ".env", ".properties", ".xml"}},
		{Language: "manifest", Extensions: nil, Files: []string{"requirements.txt", "pyproject.toml", "package.json", "tsconfig.json", "go.mod", "pom.xml", "build.gradle", "settings.gradle", "gradle.properties"}},
		{Language: "query", Extensions: []string{".sql", ".graphql", ".gql"}},
		{Language: "script", Extensions: []string{".sh", ".bash", ".zsh", ".fish", ".ps1", ".psm1", ".bat", ".cmd"}},
		{Language: "data", Extensions: []string{".csv", ".tsv"}},
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

func candidateFiles(root string, trackedOnly bool) ([]string, int, error) {
	if trackedOnly {
		lines, err := gitLines(root, "ls-files")
		if err == nil {
			return normalizeCandidates(lines), 0, nil
		}
	}
	var files []string
	skipped := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), ".git/") {
			skipped++
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	return normalizeCandidates(files), skipped, err
}

func normalizeCandidates(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(filepath.ToSlash(line))
		if line != "" {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func previousByPath(idx *Index) map[string]FileEntry {
	out := map[string]FileEntry{}
	if idx == nil {
		return out
	}
	for _, f := range idx.Files {
		out[f.Path] = f
	}
	return out
}

func chunksForFile(idx *Index, path string) []Chunk {
	if idx == nil {
		return nil
	}
	var out []Chunk
	for _, ch := range idx.Chunks {
		if ch.Path == path {
			out = append(out, ch)
		}
	}
	return out
}

func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func gitLines(repo string, args ...string) ([]string, error) {
	out, err := gitBytes(repo, args...)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func gitBytes(repo string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, nil
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true, nil
		}
	}
	return false, nil
}

func shouldSkipDir(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, ".agentdb233-") {
		return true
	}
	switch lower {
	case ".git", ".hg", ".svn", "node_modules", "vendor", "bin", "obj", "dist", "build", "target", ".next", ".vite", ".turbo", ".cache", "coverage", "__pycache__", ".venv", "venv", ".gradle", ".idea", ".vs":
		return true
	default:
		return false
	}
}

func DefaultSkipDirs() []string {
	return []string{".agentdb233-*", ".git", ".hg", ".svn", "node_modules", "vendor", "bin", "obj", "dist", "build", "target", ".next", ".vite", ".turbo", ".cache", "coverage", "__pycache__", ".venv", "venv", ".gradle", ".idea", ".vs"}
}

func DefaultSkipFiles() []string {
	return []string{"*.min.js", "*.min.css", "*.map", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lockb", "go.sum", "poetry.lock", "cargo.lock", "gradle.lockfile"}
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
