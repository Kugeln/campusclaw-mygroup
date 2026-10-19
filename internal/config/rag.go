package config

import (
	"campusclaw/internal/rag"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type RAG struct {
	Enabled   bool
	Embedding rag.Embedding
	Vectors   rag.VectorClient
	Dialogue  rag.DialogueClient
}

func LoadRAG(get func(string) string) (RAG, error) {
	c := RAG{}
	enabled := get("RAG_ENABLED")
	if enabled == "" || enabled == "false" {
		return c, nil
	}
	if enabled != "true" {
		return c, fmt.Errorf("invalid RAG_ENABLED")
	}
	c.Enabled = true
	for _, key := range []string{"QDRANT_URL", "EMBEDDING_BASE_URL", "EMBEDDING_API_KEY", "EMBEDDING_MODEL", "EMBEDDING_DIMENSION", "DIALOGUE_BASE_URL", "DIALOGUE_API_KEY", "DIALOGUE_MODEL"} {
		if get(key) == "" {
			return c, fmt.Errorf("missing required configuration: %s", key)
		}
	}
	for _, key := range []string{"QDRANT_URL", "EMBEDDING_BASE_URL", "DIALOGUE_BASE_URL"} {
		u, e := url.Parse(get(key))
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("invalid %s", key)
		}
	}
	dim, e := strconv.Atoi(get("EMBEDDING_DIMENSION"))
	if e != nil || dim <= 0 {
		return c, fmt.Errorf("invalid EMBEDDING_DIMENSION")
	}
	format := get("EMBEDDING_API_FORMAT")
	if format == "" {
		format = "ark_multimodal"
	}
	if format != "ark_multimodal" && format != "openai" {
		return c, fmt.Errorf("invalid EMBEDDING_API_FORMAT")
	}
	collection := get("QDRANT_COLLECTION")
	if collection == "" {
		collection = "campusclaw_chunks"
	}
	if strings.ContainsAny(collection, "/.? ") {
		return c, fmt.Errorf("invalid QDRANT_COLLECTION")
	}
	c.Embedding = rag.Embedding{Gateway: rag.Gateway{Base: get("EMBEDDING_BASE_URL"), Key: get("EMBEDDING_API_KEY"), Model: get("EMBEDDING_MODEL")}, Dimension: dim, Format: format}
	c.Vectors = rag.VectorClient{Gateway: rag.Gateway{Base: get("QDRANT_URL")}, Dimension: dim, Collection: collection}
	c.Dialogue = rag.DialogueClient{Gateway: rag.Gateway{Base: get("DIALOGUE_BASE_URL"), Key: get("DIALOGUE_API_KEY"), Model: get("DIALOGUE_MODEL")}}
	return c, nil
}
