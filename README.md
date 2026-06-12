# agentdb233

Agent external brain volume: shared project knowledge base for Codex, Claude Code, Cline, OpenCode and team agents.

`agentdb233` manages project memory, team norms, retrieval-friendly content, Git branches/versions, skills and MCP endpoints. It is built for code projects, games, writers and massive asset industries.

Main goal: stop agents from reading huge full documents every turn. Build a compact code/document index once, then retrieve only useful slices with file path, language, symbol and line range.

Native platform target:

- Windows
- macOS
- Linux

Native language index:

- C# (`.cs`)
- C/C++ (`.c`, `.h`, `.cc`, `.cpp`, `.hpp`)
- Go (`.go`)
- TypeScript / JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`)
- Java (`.java`)
- Kotlin (`.kt`, `.kts`)
- Python (`.py`)
- Rust / Ruby / PHP / Swift / Lua
- HTML / CSS / Vue / Svelte / Astro
- Docs (`.md`, `.mdx`, `.markdown`, `.rst`, `.adoc`, `.txt`)
- Config and manifests (`.json`, `.yaml`, `.toml`, `.xml`, `.properties`, `package.json`, `go.mod`, `Dockerfile`)
- Scripts (`.sh`, `.ps1`, `.bat`, `.cmd`)

## Install

Server:

```bash
curl -fsSL https://raw.githubusercontent.com/neko233-com/agentdb233/main/scripts/install-server.sh | bash
```

```powershell
iwr -useb https://raw.githubusercontent.com/neko233-com/agentdb233/main/scripts/install-server.ps1 | iex
```

Local build:

```bash
go install ./cmd/agentdb233-server
agentdb233-server start
```

Default URL: `http://127.0.0.1:23390`

Default data dir:

- env `AGENTDB233_DATA`
- otherwise `~/.agentdb233-server`

## Core API

```bash
curl http://127.0.0.1:23390/healthz
curl http://127.0.0.1:23390/api/status
curl http://127.0.0.1:23390/api/git/refs?repo=/path/to/repo
curl http://127.0.0.1:23390/api/knowledge?project=demo
curl http://127.0.0.1:23390/api/skills
curl http://127.0.0.1:23390/api/mcp
```

## Code Index

Build index:

```bash
curl -X POST http://127.0.0.1:23390/api/index/build \
  -H 'content-type: application/json' \
  -d '{"project":"demo","repo":"/path/to/repo","tracked_only":true,"incremental":true}'
```

Index a Git ref without switching worktree:

```bash
curl -X POST http://127.0.0.1:23390/api/index/build \
  -H 'content-type: application/json' \
  -d '{"project":"demo-main","repo":"/path/to/repo","ref":"main"}'
```

Search compact context:

```bash
curl 'http://127.0.0.1:23390/api/index/search?project=demo&q=auth%20middleware&limit=10'
```

Result includes `path`, `language`, `symbol`, `start_line`, `end_line`, `text`, `score`.

Inspect supported languages and default ignore rules:

```bash
curl http://127.0.0.1:23390/api/index/languages
```

Build result includes `stats` (`indexed_files`, `skipped_files`, `chunks`) and indexed file metadata. Generated/minified/lock/binary files and common cache/build folders are skipped by default.

Get one shared compact context pack for any AI:

```bash
curl 'http://127.0.0.1:23390/api/context/pack?project=demo&q=auth%20middleware&budget=8000'
```

Import shared norms once from project docs (`AGENTS.md`, `CLAUDE.md`, `.clinerules`, `.cursorrules`, `README.md`, `CONTRIBUTING.md`):

```bash
curl -X POST http://127.0.0.1:23390/api/norms/import \
  -H 'content-type: application/json' \
  -d '{"project":"demo","repo":"/path/to/repo"}'
```

## Knowledge

Add entry:

```bash
curl -X POST http://127.0.0.1:23390/api/knowledge \
  -H 'content-type: application/json' \
  -d '{"project":"demo","kind":"norm","title":"Branch rule","body":"feature branches use feat/<name>","tags":["team","git"],"git":{"repo":"/repo","branch":"main","commit":"abc"}}'
```

Search:

```bash
curl 'http://127.0.0.1:23390/api/knowledge?project=demo&q=branch'
```

## Git Native

`agentdb233` reads branches, tags, commits and worktree status directly from Git:

- `/api/git/refs?repo=...`
- `/api/git/commits?repo=...&ref=main&limit=50`
- `/api/git/status?repo=...`

Knowledge entries can attach `repo`, `branch`, `tag`, `commit`, `version` fields.

## Skill / MCP Registry

Register skills:

```bash
curl -X POST http://127.0.0.1:23390/api/skills \
  -H 'content-type: application/json' \
  -d '{"name":"game-lore","description":"World/lore memory rules","path":".codex/skills/game-lore/SKILL.md","tags":["game","writer"]}'
```

Register MCP:

```bash
curl -X POST http://127.0.0.1:23390/api/mcp \
  -H 'content-type: application/json' \
  -d '{"name":"unity","command":"mcp-unity","args":["--stdio"],"tags":["game"]}'
```

## CLI

```bash
agentdb233-server serve
agentdb233-server start
agentdb233-server status
agentdb233-server stop
agentdb233-server set-port 32390
agentdb233-server enable-autostart
agentdb233-server mcp
agentdb233-server version
```

## One Brain For All AI

Do not maintain separate docs for each agent/CLI. Put project facts and norms into `agentdb233` once, then connect every AI to the same HTTP API or MCP server.

MCP stdio:

```bash
agentdb233-server mcp
```

Tools:

- `build_index`
- `search_context`
- `git_refs`
