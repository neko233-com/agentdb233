---
name: agentdb233
description: Use when Codex should share project knowledge, norms, code context, Git refs, skills, MCP endpoints, or agent memory through a single agentdb233 external brain instead of reading and maintaining separate per-agent documentation. Trigger for agentdb233, shared AI brain, context pack, build/search project index, import AGENTS.md/CLAUDE.md/.clinerules norms, or connect Codex/Claude/Cline/OpenCode to one common knowledge base.
---

# agentdb233

Use one shared brain. Do not maintain separate Codex/Claude/Cline/OpenCode docs when agentdb233 is available.

## Quick Workflow

1. Ensure server exists:

```bash
agentdb233-server status
```

2. Build or refresh project index:

```bash
agentdb233-server start
curl -X POST http://127.0.0.1:23390/api/index/build \
  -H 'content-type: application/json' \
  -d '{"project":"PROJECT","repo":"REPO","tracked_only":true,"incremental":true}'
```

3. Import shared norms once:

```bash
curl -X POST http://127.0.0.1:23390/api/norms/import \
  -H 'content-type: application/json' \
  -d '{"project":"PROJECT","repo":"REPO"}'
```

4. Retrieve compact context:

```bash
curl 'http://127.0.0.1:23390/api/context/pack?project=PROJECT&q=QUERY&budget=8000'
```

## MCP

Prefer MCP when host supports it:

```bash
agentdb233-server mcp
```

Tools:

- `build_index`: build shared repo index.
- `search_context`: return compact context pack.
- `git_refs`: list branches/tags.

## Rules

- Use `tracked_only=true` for normal code repos.
- Use `incremental=true` after first index.
- Use `ref` when context must match a branch/tag/commit without switching worktree.
- Prefer `/api/context/pack` over reading whole docs.
- Add durable facts to `/api/knowledge`; do not scatter new norms across agent-specific files.
- Keep per-agent files as thin pointers to agentdb233 when possible.
