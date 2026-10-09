package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ToolCalls = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "design_mcp_tool_calls_total",
			Help: "Tool invocations.",
		},
		[]string{"tool"},
	)

	HitScore = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "design_mcp_top_hit_score",
			Help:    "Top hit cosine similarity per search.",
			Buckets: []float64{0.2, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9},
		},
		[]string{"tool"},
	)
)

func init() {
	prometheus.MustRegister(ToolCalls, HitScore)
}

func Handler() http.Handler { return promhttp.Handler() }