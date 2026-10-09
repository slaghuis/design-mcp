package search

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

type Hit struct {
	Score       float32  `json:"score"`
	Source      string   `json:"source"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	ADRID       string   `json:"adr_id,omitempty"`
	Status      string   `json:"status,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SectionPath string   `json:"section_path,omitempty"`
	Heading     string   `json:"heading,omitempty"`
	Text        string   `json:"text"`
	StartLine   int64    `json:"start_line"`
	EndLine     int64    `json:"end_line"`
}

type Filters struct {
	Source string
	Kind   string
	ADRID  string
	Status string
	Tag    string
}

type Store struct {
	client     *qdrant.Client
	collection string
}

func NewStore(host string, port int, collection string) (*Store, error) {
	c, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		return nil, err
	}
	return &Store{client: c, collection: collection}, nil
}

func buildFilter(f Filters) *qdrant.Filter {
	var must []*qdrant.Condition
	if f.Source != "" {
		must = append(must, qdrant.NewMatch("source", f.Source))
	}
	if f.Kind != "" {
		must = append(must, qdrant.NewMatch("kind", f.Kind))
	}
	if f.ADRID != "" {
		must = append(must, qdrant.NewMatch("adr_id", f.ADRID))
	}
	if f.Status != "" {
		must = append(must, qdrant.NewMatch("status", f.Status))
	}
	if f.Tag != "" {
		must = append(must, qdrant.NewMatch("tags", f.Tag))
	}
	if len(must) == 0 {
		return nil
	}
	return &qdrant.Filter{Must: must}
}

func (s *Store) Search(ctx context.Context, vec []float32, limit uint64, f Filters) ([]Hit, error) {
	resp, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collection,
		Query:          qdrant.NewQuery(vec...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		Filter:         buildFilter(f),
	})
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return hitsFrom(resp), nil
}

func hitsFrom(resp []*qdrant.ScoredPoint) []Hit {
	out := make([]Hit, 0, len(resp))
	for _, p := range resp {
		pl := p.Payload
		h := Hit{
			Score: p.Score,
			Source: str(pl, "source"), Path: str(pl, "path"),
			Kind: str(pl, "kind"), Title: str(pl, "title"),
			ADRID: str(pl, "adr_id"), Status: str(pl, "status"),
			SectionPath: str(pl, "section_path"), Heading: str(pl, "heading"),
			Text:      str(pl, "text"),
			StartLine: integer(pl, "start_line"),
			EndLine:   integer(pl, "end_line"),
		}
		h.Tags = strs(pl, "tags")
		out = append(out, h)
	}
	return out
}

// ScrollByPath returns all chunks of a document in order.
func (s *Store) ScrollByPath(ctx context.Context, source, path string) ([]Hit, error) {
	limit := uint32(256)
	resp, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewMatch("source", source),
				qdrant.NewMatch("path", path),
			},
		},
		Limit:       &limit,
		WithPayload: qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Hit, 0, len(resp))
	for _, p := range resp {
		pl := p.Payload
		out = append(out, Hit{
			Source: str(pl, "source"), Path: str(pl, "path"),
			Kind: str(pl, "kind"), Title: str(pl, "title"),
			ADRID: str(pl, "adr_id"), Status: str(pl, "status"),
			SectionPath: str(pl, "section_path"), Heading: str(pl, "heading"),
			Text:      str(pl, "text"),
			StartLine: integer(pl, "start_line"),
			EndLine:   integer(pl, "end_line"),
		})
	}
	return out, nil
}

// ListADRs returns one representative chunk per ADR, filtered by status.
func (s *Store) ListADRs(ctx context.Context, status, source string, limit uint32) ([]Hit, error) {
	flt := &qdrant.Filter{
		Must: []*qdrant.Condition{qdrant.NewMatch("kind", "adr")},
	}
	if status != "" {
		flt.Must = append(flt.Must, qdrant.NewMatch("status", status))
	}
	if source != "" {
		flt.Must = append(flt.Must, qdrant.NewMatch("source", source))
	}

	resp, err := s.client.Scroll(ctx, &qdrant.ScrollPoints{
		CollectionName: s.collection,
		Filter:         flt,
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, err
	}
	// De-duplicate by (source,path): keep first (lowest chunk_index).
	seen := map[string]bool{}
	out := []Hit{}
	for _, p := range resp {
		pl := p.Payload
		key := str(pl, "source") + "::" + str(pl, "path")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Hit{
			Source: str(pl, "source"), Path: str(pl, "path"),
			Kind: str(pl, "kind"), Title: str(pl, "title"),
			ADRID: str(pl, "adr_id"), Status: str(pl, "status"),
			Tags: strs(pl, "tags"),
		})
	}
	return out, nil
}

func str(m map[string]*qdrant.Value, k string) string {
	if v, ok := m[k]; ok {
		return v.GetStringValue()
	}
	return ""
}

func integer(m map[string]*qdrant.Value, k string) int64 {
	if v, ok := m[k]; ok {
		return v.GetIntegerValue()
	}
	return 0
}

func strs(m map[string]*qdrant.Value, k string) []string {
	if v, ok := m[k]; ok {
		lst := v.GetListValue()
		if lst == nil {
			return nil
		}
		out := make([]string, 0, len(lst.Values))
		for _, item := range lst.Values {
			out = append(out, item.GetStringValue())
		}
		return out
	}
	return nil
}