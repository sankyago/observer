package rules

import (
	"sync"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

type Cache struct {
	mu    sync.RWMutex
	index map[uuid.UUID][]models.Rule
}

func NewCache() *Cache {
	return &Cache{index: map[uuid.UUID][]models.Rule{}}
}

// Replace atomically swaps the cache contents. Disabled rules are excluded.
func (c *Cache) Replace(rules []models.Rule) {
	idx := make(map[uuid.UUID][]models.Rule, len(rules))
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		idx[r.DeviceID] = append(idx[r.DeviceID], r)
	}
	c.mu.Lock()
	c.index = idx
	c.mu.Unlock()
}

// GetByDevice returns a snapshot slice (safe to iterate).
func (c *Cache) GetByDevice(deviceID uuid.UUID) []models.Rule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	src := c.index[deviceID]
	if len(src) == 0 {
		return nil
	}
	out := make([]models.Rule, len(src))
	copy(out, src)
	return out
}
