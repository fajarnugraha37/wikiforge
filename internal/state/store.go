package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fajarnugraha37/wikiforge/internal/model"
)

type Store struct {
	Path string
	mu   sync.Mutex
}

func (s *Store) Load() (model.RunState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.RunState{}, nil
		}
		return model.RunState{}, err
	}
	var st model.RunState
	if err := json.Unmarshal(b, &st); err != nil {
		return st, err
	}
	return st, nil
}

func (s *Store) Save(st model.RunState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
