package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/slaghuis/design-mcp/internal/metrics"
	"github.com/slaghuis/design-mcp/internal/search"
)

func RegisterSearchDocs(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("search_docs",
		mcp.WithDescription(
			"Broad semantic search over all design docs, runbooks, READMEs, and ADRs. "+
				"Use this for general knowledge queries where the answer may not be in an ADR."),
		mcp.WithString("query", mcp.Required(),
			mcp.Description("Natural-language query.")),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 8, cap 25).")),
		mcp.WithString("kind",
			mcp.Description("Filter by doc kind: adr | doc | runbook | readme.")),
		mcp.WithString("source",
			mcp.Description("Filter by source name.")),
		mcp.WithString("tag",
			mcp.Description("Filter by tag.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := uint64(req.GetFloat("limit", 8))
		if limit == 0 || limit > 25 {
			limit = 8
		}

		// Record Metrics
		metrics.ToolCalls.WithLabelValues("search_docs").Inc()

		vec, cached := d.Cache.Get(d.EmbedModel, q)
		if !cached {
			v, err := d.Embedder.Embed(ctx, q)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("embed: %v", err)), nil
			}
			d.Cache.Put(d.EmbedModel, q, v)
			vec = v
		}

		hits, err := d.Store.Search(ctx, vec, limit, search.Filters{
			Kind:   req.GetString("kind", ""),
			Source: req.GetString("source", ""),
			Tag:    req.GetString("tag", ""),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.MarshalIndent(map[string]any{
			"query": q, "count": len(hits), "hits": hits,
		}, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}