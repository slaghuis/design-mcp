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

func RegisterFindDecision(s *server.MCPServer, d Deps) {
	tool := mcp.NewTool("find_decision",
		mcp.WithDescription(
			"Find the authoritative decision on an architectural topic. "+
				"Returns the top-ranked ADR with its decision, status, and consequences. "+
				"Use this BEFORE proposing changes to messaging, storage, auth, "+
				"deployment, or any cross-cutting concern. If status is Deprecated or "+
				"Superseded, the response includes a pointer to the newer ADR."),
		mcp.WithString("topic", mcp.Required(),
			mcp.Description("The topic to look up, e.g. 'event bus choice' or 'JWT refresh strategy'.")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		topic, err := req.RequireString("topic")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Recorde metrics
		metrics.ToolCalls.WithLabelValues("final_decision").Inc()

		vec, cached := d.Cache.Get(d.EmbedModel, topic)
		if !cached {
			v, err := d.Embedder.Embed(ctx, topic)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("embed: %v", err)), nil
			}
			d.Cache.Put(d.EmbedModel, topic, v)
			vec = v
		}

		// Pull a handful, prefer Accepted status.
		hits, err := d.Store.Search(ctx, vec, 10, search.Filters{Kind: "adr"})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if len(hits) == 0 {
			return mcp.NewToolResultText(`{"found": false, "note": "no ADRs matched this topic"}`), nil
		}

		// Rank: Accepted > Proposed > Superseded > Deprecated,
		// then by vector score.
		best := pickBestADR(hits)

		// Fetch the full doc to extract Decision/Consequences sections.
		fullDoc, err := d.Store.ScrollByPath(ctx, best.Source, best.Path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		decision, consequences, context := extractSections(fullDoc)

		result := map[string]any{
			"found":        true,
			"adr_id":       best.ADRID,
			"title":        best.Title,
			"status":       best.Status,
			"source":       best.Source,
			"path":         best.Path,
			"topic":        topic,
			"score":        best.Score,
			"context":      context,
			"decision":     decision,
			"consequences": consequences,
			"tags":         best.Tags,
		}

		// If deprecated/superseded, note it clearly.
		if strings.EqualFold(best.Status, "Superseded") ||
			strings.EqualFold(best.Status, "Deprecated") {
			result["warning"] = "This ADR is " + best.Status +
				". Call list_adrs with status=Accepted to find the current decision."
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

func pickBestADR(hits []search.Hit) search.Hit {
	rank := func(status string) int {
		switch strings.ToLower(status) {
		case "accepted":
			return 0
		case "proposed":
			return 1
		case "superseded":
			return 2
		case "deprecated":
			return 3
		default:
			return 4
		}
	}
	best := hits[0]
	for _, h := range hits[1:] {
		if rank(h.Status) < rank(best.Status) {
			best = h
		}
	}
	return best
}

func extractSections(chunks []search.Hit) (decision, consequences, context string) {
	for _, c := range chunks {
		hl := strings.ToLower(c.Heading)
		switch {
		case strings.HasPrefix(hl, "decision"):
			decision = c.Text
		case strings.HasPrefix(hl, "consequences"), strings.HasPrefix(hl, "trade-off"):
			consequences = c.Text
		case strings.HasPrefix(hl, "context"):
			context = c.Text
		}
	}
	return
}