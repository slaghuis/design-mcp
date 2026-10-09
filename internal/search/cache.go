package search

import (
	"crypto/sha1"
	"encoding/hex"

	lru "github.com/hashicorp/golang-lru/v2"
)

type EmbedCache struct {
	c *lru.Cache[string, []float32]
}

func NewEmbedCache(size int) *EmbedCache {
	c, _ := lru.New[string, []float32](size)
	return &EmbedCache{c: c}
}

func (e *EmbedCache) key(model, text string) string {
	h := sha1.Sum([]byte(model + "::" + text))
	return hex.EncodeToString(h[:])
}

func (e *EmbedCache) Get(model, text string) ([]float32, bool) {
	return e.c.Get(e.key(model, text))
}

func (e *EmbedCache) Put(model, text string, vec []float32) {
	e.c.Add(e.key(model, text), vec)
}