package cache

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nunoferna/aegis-llm/internal/config"
	"github.com/qdrant/go-client/qdrant"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

const (
	CollectionName = "aegis_prompt_cache"
)

type ClientOptions struct {
	SaveQueueSize             int
	SaveWorkers               int
	SaveTimeout               time.Duration
	VectorSize                int
	MaxCacheableBodyBytes     int64
	MaxCacheableResponseBytes int64
	CacheEntryTTL             time.Duration
	SearchLimit               int
	CleanupInterval           time.Duration
	CleanupTimeout            time.Duration
	CleanupEnabled            bool
	IndexPayloadFields        bool
}

type saveJob struct {
	vector     []float32
	response   []byte
	model      string
	promptHash string
}

// QdrantClient wraps the official gRPC client
type QdrantClient struct {
	client                    *qdrant.Client
	saveQueue                 chan saveJob
	saveTimeout               time.Duration
	maxCacheableBodyBytes     int64
	maxCacheableResponseBytes int64
	cacheEntryTTL             time.Duration
	searchLimit               int
	cleanupInterval           time.Duration
	cleanupTimeout            time.Duration
	cleanupEnabled            bool
	indexPayloadFields        bool
	cleanupStop               chan struct{}
	searchOverride            func(context.Context, []float32, string, string) (bool, []byte)
	enqueueOverride           func([]float32, []byte, string, string) bool
	cleanupRunsCounter        metric.Int64Counter
	cleanupDeletedCounter     metric.Int64Counter
	cleanupErrorCounter       metric.Int64Counter
	cleanupDurationHistogram  metric.Float64Histogram
	wg                        sync.WaitGroup
	mu                        sync.RWMutex
	isClosed                  bool
}

// NewQdrantClient connects to the vector DB and ensures the collection exists
func NewQdrantClient(host string, port int, opts ClientOptions) (*QdrantClient, error) {
	if opts.SaveQueueSize <= 0 {
		opts.SaveQueueSize = config.DefaultCacheSaveQueueSize
	}
	if opts.SaveWorkers <= 0 {
		opts.SaveWorkers = config.DefaultCacheSaveWorkers
	}
	if opts.SaveTimeout <= 0 {
		opts.SaveTimeout = config.DefaultCacheSaveTimeout
	}
	if opts.MaxCacheableBodyBytes <= 0 {
		opts.MaxCacheableBodyBytes = config.DefaultMaxCacheableBody
	}
	if opts.MaxCacheableResponseBytes <= 0 {
		opts.MaxCacheableResponseBytes = config.DefaultMaxCacheableResponse
	}
	if opts.CacheEntryTTL <= 0 {
		opts.CacheEntryTTL = config.DefaultCacheEntryTTL
	}
	if opts.SearchLimit <= 0 {
		opts.SearchLimit = config.DefaultCacheSearchLimit
	}
	if opts.CleanupInterval <= 0 {
		opts.CleanupInterval = config.DefaultCacheCleanupInterval
	}
	if opts.CleanupTimeout <= 0 {
		opts.CleanupTimeout = config.DefaultCacheCleanupTimeout
	}

	vectorSize := opts.VectorSize
	if vectorSize <= 0 {
		ctx, cancel := context.WithTimeout(context.Background(), embeddingConfig.Timeout)
		detectedSize, err := GetEmbeddingVectorSize(ctx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("failed to detect embedding vector size: %w", err)
		}
		vectorSize = detectedSize
	}
	if vectorSize <= 0 {
		return nil, fmt.Errorf("invalid vector size: %d", vectorSize)
	}

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	qc := &QdrantClient{
		client:                    client,
		saveQueue:                 make(chan saveJob, opts.SaveQueueSize),
		saveTimeout:               opts.SaveTimeout,
		maxCacheableBodyBytes:     opts.MaxCacheableBodyBytes,
		maxCacheableResponseBytes: opts.MaxCacheableResponseBytes,
		cacheEntryTTL:             opts.CacheEntryTTL,
		searchLimit:               opts.SearchLimit,
		cleanupInterval:           opts.CleanupInterval,
		cleanupTimeout:            opts.CleanupTimeout,
		cleanupEnabled:            opts.CleanupEnabled,
		indexPayloadFields:        opts.IndexPayloadFields,
		cleanupStop:               make(chan struct{}),
	}
	qc.initCleanupMetrics()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.CollectionExists(ctx, CollectionName)
	if err != nil {
		return nil, err
	}

	if !exists {
		log.Printf("🧠 Initializing new Qdrant collection: %s", CollectionName)
		err = client.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: CollectionName,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     uint64(vectorSize),
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			return nil, err
		}
	} else {
		log.Printf("🧠 Connected to existing Qdrant collection: %s", CollectionName)
	}

	if qc.indexPayloadFields {
		if err := qc.ensurePayloadIndexes(ctx); err != nil {
			log.Printf("⚠️ Failed to create payload indexes: %v", err)
		}
	}

	qc.startSaveWorkers(opts.SaveWorkers)
	if qc.cleanupEnabled {
		qc.startCleanupWorker()
	}

	return qc, nil
}

func (qc *QdrantClient) initCleanupMetrics() {
	meter := otel.Meter("aegis.cache.qdrant")

	runs, err := meter.Int64Counter("aegis_cache_cleanup_runs_total")
	if err == nil {
		qc.cleanupRunsCounter = runs
	}

	deleted, err := meter.Int64Counter("aegis_cache_cleanup_deleted_total")
	if err == nil {
		qc.cleanupDeletedCounter = deleted
	}

	errors, err := meter.Int64Counter("aegis_cache_cleanup_errors_total")
	if err == nil {
		qc.cleanupErrorCounter = errors
	}

	duration, err := meter.Float64Histogram("aegis_cache_cleanup_duration_seconds")
	if err == nil {
		qc.cleanupDurationHistogram = duration
	}
}

func (qc *QdrantClient) startCleanupWorker() {
	qc.wg.Add(1)
	go func() {
		defer qc.wg.Done()
		ticker := time.NewTicker(qc.cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-qc.cleanupStop:
				return
			case <-ticker.C:
				startedAt := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), qc.cleanupTimeout)
				deleted, err := qc.cleanupExpiredEntries(ctx)
				cancel()
				qc.recordCleanupRunMetrics(context.Background(), deleted, time.Since(startedAt), err)
				if err != nil {
					log.Printf("⚠️ Failed to cleanup expired cache entries: %v", err)
				} else if deleted > 0 {
					log.Printf("🧹 Cleaned up %d expired cache entries", deleted)
				}
			}
		}
	}()
}

func (qc *QdrantClient) recordCleanupRunMetrics(ctx context.Context, deleted int64, duration time.Duration, runErr error) {
	if qc.cleanupRunsCounter != nil {
		qc.cleanupRunsCounter.Add(ctx, 1)
	}
	if deleted > 0 && qc.cleanupDeletedCounter != nil {
		qc.cleanupDeletedCounter.Add(ctx, deleted)
	}
	if runErr != nil && qc.cleanupErrorCounter != nil {
		qc.cleanupErrorCounter.Add(ctx, 1)
	}
	if qc.cleanupDurationHistogram != nil {
		qc.cleanupDurationHistogram.Record(ctx, duration.Seconds())
	}
}

func (qc *QdrantClient) cleanupExpiredEntries(ctx context.Context) (int64, error) {
	now := float64(time.Now().Unix())
	exact := true
	count, err := qc.client.Count(ctx, &qdrant.CountPoints{
		CollectionName: CollectionName,
		Filter: &qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewRange("expires_at_unix", &qdrant.Range{Lte: &now}),
			},
		},
		Exact: &exact,
	})
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}

	wait := false

	_, err = qc.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: CollectionName,
		Wait:           &wait,
		Points: qdrant.NewPointsSelectorFilter(&qdrant.Filter{
			Must: []*qdrant.Condition{
				qdrant.NewRange("expires_at_unix", &qdrant.Range{Lte: &now}),
			},
		}),
	})
	if err != nil {
		return 0, err
	}

	return int64(count), nil
}

func (qc *QdrantClient) ensurePayloadIndexes(ctx context.Context) error {
	if err := qc.createFieldIndex(ctx, "expires_at_unix", qdrant.FieldType_FieldTypeInteger); err != nil {
		return err
	}
	if err := qc.createFieldIndex(ctx, "model", qdrant.FieldType_FieldTypeKeyword); err != nil {
		return err
	}
	if err := qc.createFieldIndex(ctx, "prompt_hash", qdrant.FieldType_FieldTypeKeyword); err != nil {
		return err
	}
	return nil
}

func (qc *QdrantClient) createFieldIndex(ctx context.Context, fieldName string, fieldType qdrant.FieldType) error {
	wait := true
	_, err := qc.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: CollectionName,
		Wait:           &wait,
		FieldName:      fieldName,
		FieldType:      &fieldType,
	})
	if err == nil || isIgnorableIndexError(err) {
		return nil
	}
	return err
}

func isIgnorableIndexError(err error) bool {
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "already exists") || strings.Contains(errMsg, "duplicate")
}

func (qc *QdrantClient) startSaveWorkers(workers int) {
	for i := 0; i < workers; i++ {
		qc.wg.Add(1)
		go func() {
			defer qc.wg.Done()
			for job := range qc.saveQueue {
				ctx, cancel := context.WithTimeout(context.Background(), qc.saveTimeout)
				err := qc.SaveWithMetadata(ctx, job.vector, job.response, job.model, job.promptHash)
				cancel()
				if err != nil {
					log.Printf("⚠️ Failed to save to Qdrant: %v", err)
				} else {
					log.Println("💾 Saved new response to Qdrant cache!")
				}
			}
		}()
	}
}

// EnqueueSave submits a cache-write job without blocking request completion.
// It returns false when the queue is full.
func (qc *QdrantClient) EnqueueSave(vector []float32, response []byte) bool {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	if qc.isClosed {
		return false
	}

	job := saveJob{vector: vector, response: response}
	select {
	case qc.saveQueue <- job:
		return true
	default:
		return false
	}
}

func (qc *QdrantClient) EnqueueSaveWithMetadata(vector []float32, response []byte, model string, promptHash string) bool {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	if qc.isClosed {
		return false
	}

	job := saveJob{vector: vector, response: response, model: model, promptHash: promptHash}
	select {
	case qc.saveQueue <- job:
		return true
	default:
		return false
	}
}

func (qc *QdrantClient) search(ctx context.Context, vector []float32, model string, promptHash string) (bool, []byte) {
	if qc.searchOverride != nil {
		return qc.searchOverride(ctx, vector, model, promptHash)
	}
	return qc.Search(ctx, vector, model, promptHash)
}

func (qc *QdrantClient) enqueue(vector []float32, response []byte, model string, promptHash string) bool {
	if qc.enqueueOverride != nil {
		return qc.enqueueOverride(vector, response, model, promptHash)
	}
	return qc.EnqueueSaveWithMetadata(vector, response, model, promptHash)
}

// Search looks for a semantically similar prompt in the database.
func (qc *QdrantClient) Search(ctx context.Context, vector []float32, model string, promptHash string) (bool, []byte) {

	threshold := float32(0.90)
	limit := uint64(qc.searchLimit)

	searchResult, err := qc.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: CollectionName,
		Query:          qdrant.NewQuery(vector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		ScoreThreshold: &threshold,
	})

	if err != nil || len(searchResult) == 0 {
		return false, nil
	}

	var bestModelHit *qdrant.ScoredPoint
	for _, hit := range searchResult {
		if hit.Payload == nil {
			continue
		}

		hitModel := hit.Payload["model"].GetStringValue()
		if hitModel != model {
			continue
		}

		expiresAt := hit.Payload["expires_at_unix"].GetIntegerValue()
		if expiresAt > 0 && expiresAt <= time.Now().Unix() {
			continue
		}

		hitPromptHash := hit.Payload["prompt_hash"].GetStringValue()
		if hitPromptHash == promptHash && hit.Payload["response"].GetStringValue() != "" {
			log.Printf("🎯 QDRANT HIT! Similarity Score: %f", hit.Score)
			return true, []byte(hit.Payload["response"].GetStringValue())
		}

		if bestModelHit == nil {
			bestModelHit = hit
		}
	}

	if bestModelHit != nil {
		respString := bestModelHit.Payload["response"].GetStringValue()
		if respString != "" {
			log.Printf("🎯 QDRANT HIT! Similarity Score: %f", bestModelHit.Score)
			return true, []byte(respString)
		}
	}

	return false, nil
}

// Save stores the new vector and the LLM's response into the database.
func (qc *QdrantClient) Save(ctx context.Context, vector []float32, response []byte) error {
	return qc.SaveWithMetadata(ctx, vector, response, "", "")
}

// SaveWithMetadata stores vector, response, and policy metadata.
func (qc *QdrantClient) SaveWithMetadata(ctx context.Context, vector []float32, response []byte, model string, promptHash string) error {
	wait := false
	nowUnix := time.Now().Unix()
	expiresAt := time.Now().Add(qc.cacheEntryTTL).Unix()

	point := &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(uuid.New().String()),
		Vectors: qdrant.NewVectors(vector...),
		Payload: qdrant.NewValueMap(map[string]any{
			"response":        string(response),
			"model":           model,
			"prompt_hash":     promptHash,
			"created_at_unix": nowUnix,
			"expires_at_unix": expiresAt,
		}),
	}

	_, err := qc.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: CollectionName,
		Wait:           &wait,
		Points:         []*qdrant.PointStruct{point},
	})
	return err
}

// Close cleanly shuts down the gRPC connection
func (qc *QdrantClient) Close() {
	if qc.cleanupEnabled {
		close(qc.cleanupStop)
	}

	qc.mu.Lock()
	qc.isClosed = true
	close(qc.saveQueue)
	qc.mu.Unlock()

	qc.wg.Wait()
	qc.client.Close()
}
