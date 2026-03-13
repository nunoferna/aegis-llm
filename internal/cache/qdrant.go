package cache

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

const (
	CollectionName = "aegis_prompt_cache"
	VectorSize     = 384 // The exact vector dimension size for the 'all-minilm' model
)

// QdrantClient wraps the official gRPC client
type QdrantClient struct {
	client *qdrant.Client
}

// NewQdrantClient connects to the vector DB and ensures the collection exists
func NewQdrantClient(host string, port int) (*QdrantClient, error) {
	// 1. Connect to Qdrant via gRPC
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, err
	}

	qc := &QdrantClient{client: client}

	// 2. Ensure our caching collection exists
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
				Size:     VectorSize,
				Distance: qdrant.Distance_Cosine, // Perfect for semantic text similarity
			}),
		})
		if err != nil {
			return nil, err
		}
	} else {
		log.Printf("🧠 Connected to existing Qdrant collection: %s", CollectionName)
	}

	return qc, nil
}

// Search looks for a semantically similar prompt in the database.
func (qc *QdrantClient) Search(ctx context.Context, vector []float32) (bool, []byte) {
	// We want a minimum cosine similarity of 90%
	threshold := float32(0.90)
	limit := uint64(1)

	searchResult, err := qc.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: CollectionName,
		Query:          qdrant.NewQuery(vector...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(true),
		ScoreThreshold: &threshold,
	})

	if err != nil || len(searchResult) == 0 {
		return false, nil // Cache Miss
	}

	// Cache Hit! Extract the stored JSON response
	hit := searchResult[0]
	respString := hit.Payload["response"].GetStringValue()

	log.Printf("🎯 QDRANT HIT! Similarity Score: %f", hit.Score)
	return true, []byte(respString)
}

// Save stores the new vector and the LLM's response into the database.
func (qc *QdrantClient) Save(ctx context.Context, vector []float32, response []byte) error {
	wait := true

	point := &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(uuid.New().String()),
		Vectors: qdrant.NewVectors(vector...),
		Payload: qdrant.NewValueMap(map[string]any{
			"response": string(response),
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
	qc.client.Close()
}
