package memory

import (
	"context"
	"testing"

	"github.com/theapemachine/a2a-go/pkg/a2a"

	. "github.com/smartystreets/goconvey/convey"
)

type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1}, nil
}
func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1}
	}
	return out, nil
}

type mockVectorStore struct {
	stored []Memory
}

func (m *mockVectorStore) StoreMemory(ctx context.Context, mem Memory) (string, error) {
	m.stored = append(m.stored, mem)
	return "1", nil
}
func (m *mockVectorStore) StoreMemories(ctx context.Context, mems []Memory) error { return nil }
func (m *mockVectorStore) GetMemory(ctx context.Context, id string) (Memory, error) {
	return Memory{}, nil
}
func (m *mockVectorStore) SearchSimilar(ctx context.Context, embedding []float32, params SearchParams) ([]Memory, error) {
	return []Memory{{ID: "m1", Content: "previous"}}, nil
}
func (m *mockVectorStore) DeleteMemory(ctx context.Context, id string) error { return nil }
func (m *mockVectorStore) Ping(ctx context.Context) error                    { return nil }

func TestUnifiedMemoryInjectAndExtract(t *testing.T) {
	Convey("Given a unified memory with mock stores", t, func() {
		vs := &mockVectorStore{}
		um := NewUnifiedStore(&mockEmbedder{}, vs, nil)
		task := a2a.NewTask("tester")
		task.AddMessage("user", "u", "hello")

		Convey("When injecting memories", func() {
			err := um.InjectMemories(context.Background(), task)

			Convey("Then the task should contain injected memory", func() {
				So(err, ShouldBeNil)
				So(len(task.History), ShouldBeGreaterThan, 1)
			})
		})

		Convey("When extracting memories", func() {
			err := um.ExtractMemories(context.Background(), task)

			Convey("Then the vector store should receive the memory", func() {
				So(err, ShouldBeNil)
				So(len(vs.stored), ShouldBeGreaterThan, 0)
			})
		})
	})
}
