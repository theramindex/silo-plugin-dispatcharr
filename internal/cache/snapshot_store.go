package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultSnapshotFile = "/var/lib/continuum/plugins/silo.ramindex.dispatcharr/catalog-snapshot.json"

type SnapshotStorage interface {
	Load() (Snapshot, bool, error)
	Save(Snapshot) error
	Path() string
}

type FileSnapshotStorage struct {
	path string
}

func NewFileSnapshotStorage(path string) *FileSnapshotStorage {
	if strings.TrimSpace(path) == "" {
		path = os.Getenv("DISPATCHARR_CATALOG_SNAPSHOT_FILE")
	}
	if strings.TrimSpace(path) == "" {
		path = DefaultSnapshotFile
	}
	return &FileSnapshotStorage{path: path}
}

func (s *FileSnapshotStorage) Path() string {
	return s.path
}

func (s *FileSnapshotStorage) Load() (Snapshot, bool, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return Snapshot{}, false, nil
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, false, fmt.Errorf("decode catalog snapshot file: %w", err)
	}
	if len(snapshot.Catalog.Channels) == 0 {
		return Snapshot{}, false, nil
	}
	return snapshot, true, nil
}

func (s *FileSnapshotStorage) Save(snapshot Snapshot) error {
	if len(snapshot.Catalog.Channels) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode catalog snapshot file: %w", err)
	}
	data = append(data, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
