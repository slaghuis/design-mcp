package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/slaghuis/design-mcp/internal/metrics"
	"github.com/slaghuis/design-mcp/internal/search"
)

func RegisterGetADR(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("get_adr",
		mcp.WithDescription(
			"Fetch the full content of an ADR by its ID (e.g. '0023'). "+
				"Returns all sections in order."),
		mcp.WithString("adr_id", mcp.Required(),
			mcp.Description("ADR identifier, e.g. '0023'.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("adr_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Record Metrics
		metrics.ToolCalls.WithLabelValues("get_adrs").Inc()

		// Normalize: user may pass "23" or "0023".
		id = strings.TrimPrefix(id, "ADR-")
		id = strings.TrimPrefix(id, "adr-")
		if len(id) < 4 {
			id = strings.Repeat("0", 4-len(id)) + id
		}

		// Find any chunk with this adr_id to get its path.
		hits, err := d.Store.ListADRs(ctx, "", "", 500)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		var target *search.Hit
		for i := range hits {
			if hits[i].ADRID == id {
				target = &hits[i]
				break
			}
		}
		if target == nil {
			return mcp.NewToolResultError(fmt.Sprintf("ADR %s not found", id)), nil
		}
		full, err := d.Store.ScrollByPath(ctx, target.Source, target.Path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.MarshalIndent(map[string]any{
			"adr_id":  id,
			"title":   target.Title,
			"status":  target.Status,
			"source":  target.Source,
			"path":    target.Path,
			"tags":    target.Tags,
			"content": full,
		}, "", "  ")

		return mcp.NewToolResultText(string(b)), nil
	})
}

func RegisterGetDoc(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("get_doc",
		mcp.WithDescription("Fetch a document by source and path, returning all its chunks in order."),
		mcp.WithString("source", mcp.Required()),
		mcp.WithString("path", mcp.Required()),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		source, err := req.RequireString("source")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path, err := req.RequireString("path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		full, err := d.Store.ScrollByPath(ctx, source, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.MarshalIndent(map[string]any{
			"source": source, "path": path, "content": full,
		}, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}