package memory

import (
	"io"

	"github.com/theapemachine/a2a-go/pkg/types"
)

type Store interface {
	io.ReadWriteCloser
}

type UnifiedLongTerm struct {
	stores []Store
}

func NewUnifiedLongTerm(stores ...Store) *UnifiedLongTerm {
	return &UnifiedLongTerm{
		stores: stores,
	}
}

func (unified *UnifiedLongTerm) Remember(task *types.Task) error {
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			if part.Type == types.PartTypeText {
				for _, store := range unified.stores {
					if _, err := store.Write([]byte(part.Text)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (unified *UnifiedLongTerm) Recall(task *types.Task) error {
	for _, store := range unified.stores {
		data, err := io.ReadAll(store)
		if err != nil {
			return err
		}
		task.Artifacts = append(task.Artifacts, types.Artifact{
			Parts: []types.Part{
				{
					Type: types.PartTypeText,
					Text: string(data),
				},
			},
		})
	}
	return nil
}

func (unified *UnifiedLongTerm) Forget(task *types.Task) error {
	for _, store := range unified.stores {
		if _, err := store.Write([]byte{}); err != nil {
			return err
		}
	}
	return nil
}
