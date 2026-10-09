package main

import (
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"

	"github.com/slaghuis/design-mcp/internal/embedder"
	"github.com/slaghuis/design-mcp/internal/metrics"
	"github.com/slaghuis/design-mcp/internal/search"
	"github.com/slaghuis/design-mcp/internal/tools"
)

const instructions = `
This MCP server provides access to the organization's design knowledge:
ADRs (Architecture Decision Records), design docs, runbooks, and READMEs.

WHEN TO USE EACH TOOL

- find_decision(topic): Call this FIRST when a user asks about architecture
  choices or you are about to propose one. Returns the authoritative ADR,
  its status, decision text, and consequences. Flags deprecated or superseded
  ADRs so you don't resurrect old decisions.

- search_adrs(question): When you want multiple relevant ADRs rather than a
  single decision. Good for exploratory questions like "what have we decided
  about caching?".

- search_docs(query): Broad search across all docs including runbooks and
  READMEs. Use when the answer is unlikely to be an ADR.

- list_adrs(status=Accepted): "What are our current architectural commitments?"

- get_adr(adr_id): When you have an ADR ID (e.g. from a code comment or an
  earlier search_adrs result) and need the full text.

- get_doc(source, path): Full document retrieval when you've located the file.

- cite_source(claim): Attach citations to a claim before including it in a
  response. Use this for traceability on any design assertion.

GUIDELINES

1. Never propose architectural changes without checking find_decision first.
2. Prefer Accepted ADRs; call out Superseded/Deprecated status explicitly.
3. When summarizing, use cite_source so the user can verify.
4. Doc content is in Markdown; preserve formatting when quoting.
`

type Config struct {
	Listen string `yaml:"listen"`
	Qdrant struct {
		Host       string `yaml:"host"`
		Port       int    `yaml:"port"`
		Collection string `yaml:"collection"`
	} `yaml:"qdrant"`
	Ollama struct {
		BaseURL string `yaml:"base_url"`
		Model   string `yaml:"model"`
	} `yaml:"ollama"`
	CacheSize int `yaml:"cache_size"`
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "config path")
	flag.Parse()
	log.SetOutput(os.Stderr) // reserve stdout for anything JSON-RPC (not used in SSE but safe)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	b, err := os.ReadFile(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("yaml: %v", err)
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8767"
	}
	if cfg.CacheSize == 0 {
		cfg.CacheSize = 512
	}

	emb := embedder.NewOllama(cfg.Ollama.BaseURL, cfg.Ollama.Model)
	store, err := search.NewStore(cfg.Qdrant.Host, cfg.Qdrant.Port, cfg.Qdrant.Collection)
	if err != nil {
		log.Fatalf("qdrant: %v", err)
	}

	deps := tools.Deps{
		Embedder:   emb,
		Store:      store,
		Cache:      search.NewEmbedCache(cfg.CacheSize),
		EmbedModel: cfg.Ollama.Model,
	}

	s := server.NewMCPServer(
		"design",
		"0.1.0",
		server.WithToolCapabilities(true),
		server.WithInstructions(instructions),
	)
	tools.RegisterSearchADRs(s, deps)
	tools.RegisterSearchDocs(s, deps)
	tools.RegisterFindDecision(s, deps)
	tools.RegisterListADRs(s, deps)
	tools.RegisterGetADR(s, deps)
	tools.RegisterGetDoc(s, deps)
	tools.RegisterCiteSource(s, deps)

	sse := server.NewSSEServer(s)
	mux.Handle("/sse", sse)
	mux.Handle("/message", sse)
	mux.Handle("/metrics", metrics.Handler())

	logger.Info("design-mcp starting", "listen", cfg.Listen,
		"collection", cfg.Qdrant.Collection)
	if err := http.ListenAndServe(cfg.Listen, sse); err != nil {
		log.Fatal(err)
	}
}