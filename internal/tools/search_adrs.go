package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/slaghuis/design-mcp/internal/embedder"
	"github.com/slaghuis/design-mcp/internal/metrics"
	"github.com/slaghuis/design-mcp/internal/search"
)

type Deps struct {
	Embedder   *embedder.Ollama
	Store      *search.Store
	Cache      *search.EmbedCache
	EmbedModel string
}

func RegisterSearchADRs(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("search_adrs",
		mcp.WithDescription(
			"Semantic search over Architecture Decision Records. Use this BEFORE "+
				"proposing architectural choices to find prior decisions on the same "+
				"topic. Returns ADR title, status, and the most relevant section."),
		mcp.WithString("question", mcp.Required(),
			mcp.Description("Natural-language question or topic, e.g. "+
				"'What messaging system did we pick?' or 'retry strategy'.")),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 5, cap 15).")),
		mcp.WithString("status",
			mcp.Description("Filter by ADR status: Proposed | Accepted | Deprecated | Superseded.")),
		mcp.WithString("source",
			mcp.Description("Restrict to a specific doc source (e.g. 'central', 'myservice').")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := req.RequireString("question")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := uint64(req.GetFloat("limit", 5))
		if limit == 0 || limit > 15 {
			limit = 5
		}

		//Record metrics
		metrics.ToolCalls.WithLabelValues("search_adrs").Inc()

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
			Kind:   "adr",
			Status: req.GetString("status", ""),
			Source: req.GetString("source", ""),
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.MarshalIndent(map[string]any{
			"question": q,
			"count":    len(hits),
			"hits":     hits,
		}, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}