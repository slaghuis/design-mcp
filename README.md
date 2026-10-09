 # Design MCP

 ## Build
```
# Indexer
cd ~/code/ai-factory/design-indexer
go mod tidy
go build -o ~/.local/bin/design-indexer ./cmd/design-indexer

# MCP server
cd ~/code/ai-factory/design-mcp
go mod tidy
go build -o ~/.local/bin/design-mcp ./cmd/design-mcp
```
 ## Create the Qdrant Collection
Done automatically on first indexer run, but you can pre-create for custom config:
```
curl -X PUT http://localhost:6333/collections/design_docs \
  -H 'Content-Type: application/json' \
  -d '{"vectors": {"size": 1024, "distance": "Cosine"}}'
```

 ## First Index Run
```
~/.local/bin/design-indexer -config ~/code/ai-factory/design-indexer/config.yaml
# Logs:
# INFO index complete docs=47 chunks=312 skipped=0 duration=14.2s
```
Re-runs are fast because unchanged docs are skipped via `doc_hash`.

 ## Start MCP Server
```
~/.local/bin/design-mcp -config ~/code/ai-factory/design-mcp/config.yaml
# INFO design-mcp starting listen=:8767 collection=design_docs
```
Add a launchd plist for persistent operation, same pattern as telegram-mcp.

 ## Smoke Test
```
npx @modelcontextprotocol/inspector --transport sse http://localhost:8767/sse
```
Try:
 - find_decision(topic="messaging system") → should return your Kafka ADR.
 - list_adrs(status="Accepted") → full inventory.
 - search_docs(query="deployment runbook") → runbooks surface.

 ## Wire into Agents
Add to opencode `~/.config/opencode/opencode.json`:
```
{
  "mcp": {
    "design": {
      "type": "remote",
      "url": "http://localhost:8767/sse",
      "enabled": true
    }
  }
}
```
Cursor `~/.cursor/mcp.json`:
```
{
  "mcpServers": {
    "design": { "url": "http://localhost:8767/sse" }
  }
}
```
Claude Code:
```
claude mcp add --transport sse design http://localhost:8767/sse
```

 ## Agent Nudge
Add to your `AGENTS.md` / `CLAUDE.md` / Cursor rules:
```
## Design Decisions

Before proposing changes to messaging, storage, auth, deployment,
observability, or any cross-cutting concern:

1. Call `find_decision(topic=...)` to check the authoritative ADR.
2. If the ADR status is Superseded or Deprecated, call `list_adrs(status=Accepted)`
   to find the current decision.
3. When summarising a decision in your response, call `cite_source(claim=...)`
   and include the citation inline.
4. If no ADR covers the topic and the change is architectural, suggest the
   user author a new ADR before implementing.
```

 ## ADR Workflow (Optional Enhancement)
To close the loop, add a tiny helper that creates new ADRs with the right frontmatter. Not required for the MCP to work, but it keeps your corpus clean.
Create `~/code/ai-factory/scripts/new-adr.sh:`
```
#!/usr/bin/env bash
set -euo pipefail
ROOT="${ADR_ROOT:-$HOME/code/design-docs/adr}"
mkdir -p "$ROOT"

last=$(ls "$ROOT" 2>/dev/null | grep -E '^[0-9]{4}-' | sort | tail -1 | cut -c1-4 || echo 0000)
next=$(printf "%04d" $((10#$last + 1)))
title="${*:-untitled}"
slug=$(echo "$title" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/-\{2,\}/-/g' | sed 's/^-\|-$//g')
file="$ROOT/$next-$slug.md"

cat > "$file" <<EOF
---
id: $next
title: $title
status: Proposed
date: $(date -u +%Y-%m-%d)
deciders: [$USER]
tags: []
---

# ADR $next: $title

## Context

## Decision

## Consequences

### Positive

### Negative

## Alternatives considered
EOF

echo "created $file"
```
```
chmod +x ~/code/ai-factory/scripts/new-adr.sh
new-adr.sh "Use NATS JetStream for inter-service events"
```






