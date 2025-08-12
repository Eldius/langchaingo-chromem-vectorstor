package chromem

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"hash/fnv"

	"github.com/google/uuid"
	"github.com/philippgille/chromem-go"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"
)

var (
	_ vectorstores.VectorStore = &Storage{}
)

var (
	ErrNotADirectory = errors.New("db folder is not a directory")
)

type Storage struct {
	db     *chromem.DB
	em     embeddings.Embedder
	emFunc chromem.EmbeddingFunc
	coll   *chromem.Collection
	logger *slog.Logger
}

func New(em embeddings.Embedder, opts ...Option) (vectorstores.VectorStore, error) {
	cfg := defaultStorageOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	if stat, err := os.Stat(cfg.dbPath); err != nil {
		_ = os.MkdirAll(filepath.Dir(cfg.dbPath), 0755)
	} else if !stat.IsDir() {
		return nil, ErrNotADirectory
	}
	db, err := chromem.NewPersistentDB(cfg.dbPath, true)
	if err != nil {
		return nil, fmt.Errorf("creating chromem db: %w", err)
	}

	embeddingFunc := embeddingFunction(em)

	collection, err := db.GetOrCreateCollection(cfg.collName, nil, embeddingFunc)
	if err != nil {
		return nil, err
	}

	return &Storage{
		db:     db,
		em:     em,
		emFunc: embeddingFunc,
		coll:   collection,
		logger: cfg.logger,
	}, nil
}

func embeddingFunction(em embeddings.Embedder) chromem.EmbeddingFunc {
	return func(ctx context.Context, text string) ([]float32, error) {
		docsResult, err := em.EmbedDocuments(ctx, []string{text})
		if err != nil {
			return nil, err
		}

		var result []float32
		for _, d := range docsResult {
			result = append(result, d...)
		}

		return result, nil
	}
}

func (s *Storage) AddDocuments(ctx context.Context, docs []schema.Document, options ...vectorstores.Option) ([]string, error) {

	var ids []string
	for _, doc := range docs {
		d, err := s.em.EmbedDocuments(ctx, []string{doc.PageContent})
		if err != nil {
			return nil, fmt.Errorf("embedding document: %w", err)
		}
		md := make(map[string]string)
		for k, v := range doc.Metadata {
			md[k] = fmt.Sprintf("%v", v)
		}
		for _, bdoc := range d {
			id := s.generateID(ctx, bdoc)
			newDoc, err := chromem.NewDocument(ctx, id, md, bdoc, doc.PageContent, s.emFunc)
			if err != nil {
				return nil, fmt.Errorf("creating new document: %w", err)
			}
			if err := s.coll.AddDocument(ctx, newDoc); err != nil {
				return nil, fmt.Errorf("adding document to collName: %w", err)
			}
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (s *Storage) generateID(ctx context.Context, doc []float32) string {
	hasher := fnv.New64a()
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, doc)
	if err != nil {
		s.logger.With("error", err).WarnContext(ctx, "error generating hash")
		return uuid.NewString()
	}

	_, err = hasher.Write(buf.Bytes())
	if err != nil {
		s.logger.With("error", err).WarnContext(ctx, "error generating hash")
		return uuid.NewString()
	}

	hasher.Sum64()

	return fmt.Sprintf("%016x", hasher.Sum64())
}

func (s *Storage) SimilaritySearch(ctx context.Context, query string, numDocuments int, options ...vectorstores.Option) ([]schema.Document, error) {
	opts := defaultOptions(s)
	for _, opt := range options {
		opt(&opts)
	}

	if numDocuments > s.coll.Count() {
		numDocuments = s.coll.Count()
	}

	qem, err := opts.Embedder.EmbedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embedding query: %w", err)
	}
	qf := make(map[string]string)
	if opts.Filters != nil {
		if f, ok := opts.Filters.(map[string]string); ok {
			qf = f
		}
	}

	queryRes, err := s.coll.QueryWithOptions(ctx, chromem.QueryOptions{
		QueryText:      query,
		QueryEmbedding: qem,
		NResults:       numDocuments,
		Where:          qf,
		WhereDocument:  nil,
		Negative:       chromem.NegativeQueryOptions{},
	})
	if err != nil {
		return nil, fmt.Errorf("querying collName: %w", err)
	}
	var docs []schema.Document
	for _, res := range queryRes {
		md := make(map[string]any)
		for k, v := range queryRes[0].Metadata {
			md[k] = v
		}
		docs = append(docs, schema.Document{
			PageContent: res.Content,
			Metadata:    md,
			Score:       res.Similarity,
		})
	}
	return docs, nil
}

func defaultOptions(s *Storage) vectorstores.Options {
	return vectorstores.Options{
		NameSpace:      "",
		ScoreThreshold: 0,
		Filters:        nil,
		Embedder:       s.em,
		Deduplicater:   nil,
	}
}
