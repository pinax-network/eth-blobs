package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// MemStore is an in-memory Store for tests. It records every key touched so
// tests can assert on call counts, and can be told to fail specific keys.
type MemStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	heads   map[string]Head
	Calls   []string
	FailKey map[string]error
}

var _ Store = (*MemStore)(nil)

func NewMemStore() *MemStore {
	return &MemStore{objects: map[string][]byte{}, heads: map[string]Head{}, FailKey: map[string]error{}}
}

func (m *MemStore) record(key string) error {
	m.Calls = append(m.Calls, key)
	if err, ok := m.FailKey[key]; ok {
		return err
	}
	return nil
}

// CallsFor counts how many operations touched key.
func (m *MemStore) CallsFor(key string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, c := range m.Calls {
		if c == key {
			n++
		}
	}
	return n
}

// Has reports whether an object exists, without recording a call.
func (m *MemStore) Has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok
}

// Keys returns every stored object key, sorted.
func (m *MemStore) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.objects))
	for k := range m.objects {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (m *MemStore) get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.record(key); err != nil {
		return nil, err
	}
	v, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("%s: %w", key, ErrNotFound)
	}
	return append([]byte(nil), v...), nil
}

func (m *MemStore) put(key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.record(key); err != nil {
		return err
	}
	m.objects[key] = append([]byte(nil), data...)
	return nil
}

func (m *MemStore) GetSlot(_ context.Context, slot uint64) ([]byte, error) {
	return m.get(SlotKey(slot))
}

func (m *MemStore) PutSlot(_ context.Context, slot uint64, data []byte) error {
	return m.put(SlotKey(slot), data)
}

func (m *MemStore) DeleteSlots(_ context.Context, slots []uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, slot := range slots {
		key := SlotKey(slot)
		if err := m.record(key); err != nil {
			return err
		}
		delete(m.objects, key)
	}
	return nil
}

func (m *MemStore) GetRoot(_ context.Context, root string) (uint64, error) {
	data, err := m.get(RootKey(root))
	if err != nil {
		return 0, err
	}
	var slot uint64
	if _, err := fmt.Sscanf(string(data), "%d", &slot); err != nil {
		return 0, err
	}
	return slot, nil
}

func (m *MemStore) PutRoot(_ context.Context, root string, slot uint64) error {
	return m.put(RootKey(root), []byte(fmt.Sprintf("%d", slot)))
}

func (m *MemStore) GetHead(_ context.Context) (Head, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.record(headPrefix); err != nil {
		return Head{}, err
	}
	var best Head
	found := false
	for _, h := range m.heads {
		if !found || h.Slot > best.Slot {
			best, found = h, true
		}
	}
	if !found {
		return Head{}, fmt.Errorf("head: %w", ErrNotFound)
	}
	return best, nil
}

func (m *MemStore) PutHead(_ context.Context, writer string, head Head) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.record(HeadKey(writer)); err != nil {
		return err
	}
	head.Writer = writer
	m.heads[writer] = head
	return nil
}

func (m *MemStore) GetCursor(_ context.Context, writer string) (string, error) {
	data, err := m.get(CursorKey(writer))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *MemStore) PutCursor(_ context.Context, writer string, cursor string) error {
	return m.put(CursorKey(writer), []byte(cursor))
}
