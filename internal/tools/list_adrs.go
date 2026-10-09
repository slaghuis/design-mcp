package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/slaghuis/design-mcp/internal/metrics"
)

func RegisterListADRs(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("list_adrs",
		mcp.WithDescription(
			"List all known ADRs, optionally filtered by status or source. "+
				"Useful for 'What have we decided?' and 'What's deprecated?' queries."),
		mcp.WithString("status",
			mcp.Description("Filter by status.")),
		mcp.WithString("source",
			mcp.Description("Filter by source.")),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 100, cap 500).")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

		//Record metrics
		metrics.ToolCalls.WithLabelValues("list_adrs").Inc()

		limit := uint32(req.GetFloat("limit", 100))
		if limit == 0 || limit > 500 {
			limit = 100
		}
		hits, err := d.Store.ListADRs(ctx,
			req.GetString("status", ""),
			req.GetString("source", ""),
			limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.MarshalIndent(map[string]any{
			"count": len(hits),
			"adrs":  hits,
		}, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}