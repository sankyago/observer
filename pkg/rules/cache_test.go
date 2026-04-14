package rules

import (
	"testing"

	"github.com/google/uuid"

	"github.com/observer-io/observer/pkg/models"
)

func TestCache_GetByDevice(t *testing.T) {
	c := NewCache()
	d1 := uuid.New()
	d2 := uuid.New()
	r1 := models.Rule{ID: uuid.New(), DeviceID: d1, Enabled: true}
	r2 := models.Rule{ID: uuid.New(), DeviceID: d1, Enabled: true}
	r3 := models.Rule{ID: uuid.New(), DeviceID: d2, Enabled: true}
	r4 := models.Rule{ID: uuid.New(), DeviceID: d1, Enabled: false} // skipped

	c.Replace([]models.Rule{r1, r2, r3, r4})

	got := c.GetByDevice(d1)
	if len(got) != 2 {
		t.Fatalf("d1 rules: got %d want 2", len(got))
	}
	if len(c.GetByDevice(d2)) != 1 {
		t.Error("d2 rules")
	}
	if len(c.GetByDevice(uuid.New())) != 0 {
		t.Error("unknown device should return 0")
	}
}
