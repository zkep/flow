package flow

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrCheckpointNotFound    = errors.New("checkpoint not found")
	ErrInvalidCheckpoint     = errors.New("invalid checkpoint data")
	ErrCheckpointInvalidType = errors.New("checkpoint type mismatch")
	ErrValueNotSerializable  = errors.New("value is not serializable")
)

type FlowCheckpointable interface {
	SaveCheckpoint() (*Checkpoint, error)
	LoadCheckpoint(checkpoint *Checkpoint) error
	SaveToStore(store CheckpointStore, key string) error
	LoadFromStore(store CheckpointStore, key string) error
	Reset()
}

type CheckpointStore interface {
	Save(key string, checkpoint *Checkpoint) error
	Load(key string) (*Checkpoint, error)
	Delete(key string) error
	List() ([]string, error)
}

type Checkpoint struct {
	ID        string             `json:"id"`
	Type      string             `json:"type"`
	CreatedAt time.Time          `json:"created_at"`
	Version   int                `json:"version"`
	State     FlowState          `json:"state"`
	Data      FlowCheckpointData `json:"data"`
	Metadata  map[string]string  `json:"metadata,omitempty"`
}

type FlowState int

const (
	FlowStateIdle FlowState = iota
	FlowStateRunning
	FlowStatePaused
	FlowStateCompleted
	FlowStateFailed
)

const (
	CheckpointTypeGraph = "graph"
	CheckpointTypeChain = "chain"
	defaultDirPerm      = 0750
	defaultFilePerm     = 0600
)

type FlowCheckpointData struct {
	Steps   []StepState    `json:"steps,omitempty"`
	Current int            `json:"current,omitempty"`
	Values  []any          `json:"values,omitempty"`
	Error   string         `json:"error,omitempty"`
	Extra   map[string]any `json:"extra,omitempty"`
}

type StepState struct {
	Name     string `json:"name"`
	Status   int    `json:"status"`
	Executed bool   `json:"executed"`
}

func NewCheckpoint(flowType string) *Checkpoint {
	return &Checkpoint{
		Type:    flowType,
		Version: 1,
		State:   FlowStateIdle,
		Data: FlowCheckpointData{
			Steps:  make([]StepState, 0),
			Values: make([]any, 0),
		},
	}
}

func (c *Checkpoint) SetMetadata(key, value string) {
	if c.Metadata == nil {
		c.Metadata = make(map[string]string)
	}
	c.Metadata[key] = value
}

func (c *Checkpoint) GetMetadata(key string) (string, bool) {
	if c.Metadata == nil {
		return "", false
	}
	v, ok := c.Metadata[key]
	return v, ok
}

type FileCheckpointStore struct {
	dir string
	mu  sync.RWMutex
}

func NewFileCheckpointStore(dir string) (*FileCheckpointStore, error) {
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return nil, err
	}
	return &FileCheckpointStore{dir: dir}, nil
}

func (s *FileCheckpointStore) Save(key string, checkpoint *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoint.ID = key
	checkpoint.CreatedAt = time.Now()

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath(key), data, defaultFilePerm)
}

func (s *FileCheckpointStore) Load(key string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.filePath(key)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrCheckpointNotFound
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, err
	}

	return &checkpoint, nil
}

func (s *FileCheckpointStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Clean(s.filePath(key))
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ErrCheckpointNotFound
	}

	return os.Remove(path)
}

func (s *FileCheckpointStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			name := entry.Name()
			keys = append(keys, name[:len(name)-5])
		}
	}
	return keys, nil
}

func (s *FileCheckpointStore) filePath(key string) string {
	return filepath.Join(s.dir, key+".json")
}

type MemoryCheckpointStore struct {
	data map[string]*Checkpoint
	mu   sync.RWMutex
}

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{
		data: make(map[string]*Checkpoint),
	}
}

func (s *MemoryCheckpointStore) Save(key string, checkpoint *Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	checkpoint.ID = key
	checkpoint.CreatedAt = time.Now()
	s.data[key] = checkpoint
	return nil
}

func (s *MemoryCheckpointStore) Load(key string) (*Checkpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checkpoint, ok := s.data[key]
	if !ok {
		return nil, ErrCheckpointNotFound
	}
	return checkpoint, nil
}

func (s *MemoryCheckpointStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[key]; !ok {
		return ErrCheckpointNotFound
	}
	delete(s.data, key)
	return nil
}

func (s *MemoryCheckpointStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}
