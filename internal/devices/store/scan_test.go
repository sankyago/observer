package store

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeScanner implements the scanner interface for unit-testing scan().
type fakeScanner struct {
	row []any // values to copy into dest in order
	err error
}

func (f *fakeScanner) Scan(dest ...any) error {
	if f.err != nil {
		return f.err
	}
	for i, d := range dest {
		switch v := d.(type) {
		case *uuid.UUID:
			*v = f.row[i].(uuid.UUID)
		case *string:
			*v = f.row[i].(string)
		case *time.Time:
			*v = f.row[i].(time.Time)
		}
	}
	return nil
}

func TestScan_HappyPath(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)

	fs := &fakeScanner{row: []any{id, "pump-1", "tok-abc", now, now}}
	d, err := scan(fs)
	require.NoError(t, err)
	assert.Equal(t, id, d.ID)
	assert.Equal(t, "pump-1", d.Name)
	assert.Equal(t, "tok-abc", d.Token)
	assert.Equal(t, now, d.CreatedAt)
	assert.Equal(t, now, d.UpdatedAt)
}

func TestScan_ErrNoRows_MapsToErrNotFound(t *testing.T) {
	fs := &fakeScanner{err: pgx.ErrNoRows}
	_, err := scan(fs)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestScan_OtherError_Propagates(t *testing.T) {
	sentinel := errors.New("db error")
	fs := &fakeScanner{err: sentinel}
	_, err := scan(fs)
	assert.ErrorIs(t, err, sentinel)
}
