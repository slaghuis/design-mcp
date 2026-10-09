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

func RegisterCiteSource(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("cite_source",
		mcp.WithDescription(
			"Given a claim or paraphrase, find the source document(s) that back it up. "+
				"Returns up to 3 citations with doc path and the exact section. "+
				"Use this when summarizing or paraphrasing design decisions in a response."),
		mcp.WithString("claim", mcp.Required(),
			mcp.Description("The statement to attribute.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		claim, err := req.RequireString("claim")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		
		metrics.ToolCalls.WithLabelValues("cite_source").Inc()

		vec, cached := d.Cache.Get(d.EmbedModel, claim)
		if !cached {
			v, err := d.Embedder.Embed(ctx, claim)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("embed: %v", err)), nil
			}
			d.Cache.Put(d.EmbedModel, claim, v)
			vec = v
		}
		hits, err := d.Store.Search(ctx, vec, 3, search.Filters{})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		type citation struct {
			Source      string  `json:"source"`
			Path        string  `json:"path"`
			Title       string  `json:"title"`
			SectionPath string  `json:"section_path,omitempty"`
			Snippet     string  `json:"snippet"`
			Score       float32 `json:"score"`
		}
		citations := make([]citation, 0, len(hits))
		for _, h := range hits {
			citations = append(citations, citation{
				Source: h.Source, Path: h.Path, Title: h.Title,
				SectionPath: h.SectionPath,
				Snippet:     truncate(h.Text, 300),
				Score:       h.Score,
			})
		}
		b, _ := json.MarshalIndent(map[string]any{
			"claim": claim, "citations": citations,
		}, "", "  ")

		return mcp.NewToolResultText(string(b)), nil
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}