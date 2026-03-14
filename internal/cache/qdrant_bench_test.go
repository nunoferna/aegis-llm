package cache

import "testing"

const enqueueBenchmarkVectorSize = 384

func BenchmarkEnqueueSave(b *testing.B) {
	qc := &QdrantClient{saveQueue: make(chan saveJob, 1)}
	vector := make([]float32, enqueueBenchmarkVectorSize)
	response := []byte(`{"response":"ok"}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if !qc.EnqueueSave(vector, response) {
			b.Fatal("enqueue should succeed when queue is drained every iteration")
		}
		<-qc.saveQueue
	}
}

func BenchmarkEnqueueSaveQueueFull(b *testing.B) {
	qc := &QdrantClient{saveQueue: make(chan saveJob, 1)}
	vector := make([]float32, enqueueBenchmarkVectorSize)
	response := []byte(`{"response":"ok"}`)

	if !qc.EnqueueSave(vector, response) {
		b.Fatal("failed to prefill queue")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = qc.EnqueueSave(vector, response)
	}
}

func TestEnqueueSaveReturnsFalseWhenQueueIsFull(t *testing.T) {
	qc := &QdrantClient{saveQueue: make(chan saveJob, 1)}
	vector := make([]float32, enqueueBenchmarkVectorSize)
	response := []byte(`{"response":"ok"}`)

	if !qc.EnqueueSave(vector, response) {
		t.Fatal("expected first enqueue to succeed")
	}
	if qc.EnqueueSave(vector, response) {
		t.Fatal("expected second enqueue to fail when queue is full")
	}
}
