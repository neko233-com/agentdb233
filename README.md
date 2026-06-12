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
- Go (`.go`)
- TypeScript / JavaScript (`.ts`, `.tsx`, `.js`, `.jsx`)
- Java (`.java`)
- Kotlin (`.kt`, `.kts`)
- Python (`.py`)
- Docs (`.md`, `.mdx`, `.txt`)

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
  -d '{"project":"demo","repo":"/path/to/repo"}'
```

Search compact context:

```bash
curl 'http://127.0.0.1:23390/api/index/search?project=demo&q=auth%20middleware&limit=10'
```

Result includes `path`, `language`, `symbol`, `start_line`, `end_line`, `text`, `score`.

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
agentdb233-server version
```
