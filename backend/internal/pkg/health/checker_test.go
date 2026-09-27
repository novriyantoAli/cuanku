package health_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/novriyantoAli/cuanku/backend/internal/pkg/health"
	"github.com/novriyantoAli/cuanku/backend/internal/pkg/testutil"
)

// errUnreachable is the shape of failure a real driver reports when the
// database is not listening.
var errUnreachable = errors.New("connection refused")

// stubPinger stands in for PostgreSQL: the checker only needs "reachable or not",
// which is the whole point of the Pinger seam.
type stubPinger struct {
	err    error
	called bool
}

func (s *stubPinger) Ping(context.Context) error {
	s.called = true
	return s.err
}

func newChecker(t *testing.T, pinger health.Pinger) *health.Checker {
	t.Helper()
	return health.NewChecker(pinger, health.CheckerConfig{Service: "cuanku-api", Version: "0.1.0"}, testutil.Logger(t))
}

func TestCheckReportsOKWhenDatabaseReachable(t *testing.T) {
	pinger := &stubPinger{}

	status := newChecker(t, pinger).Check(context.Background())

	require.True(t, pinger.called, "the checker must actually probe the dependency")
	assert.True(t, status.Healthy())
	assert.Equal(t, health.StatusOK, status.Status)
	assert.Equal(t, health.ComponentUp, status.Database)
	assert.Equal(t, "cuanku-api", status.Service)
	assert.Equal(t, "0.1.0", status.Version)
	assert.False(t, status.CheckedAt.IsZero())
}

func TestCheckReportsDegradedWhenDatabaseDown(t *testing.T) {
	pinger := &stubPinger{err: errUnreachable}

	status := newChecker(t, pinger).Check(context.Background())

	assert.False(t, status.Healthy())
	assert.Equal(t, health.StatusDegraded, status.Status)
	assert.Equal(t, health.ComponentDown, status.Database)
}

func TestCheckHonoursCancelledContext(t *testing.T) {
	pinger := &ctxPinger{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	status := newChecker(t, pinger).Check(ctx)

	assert.Equal(t, health.StatusDegraded, status.Status)
}

// ctxPinger fails as soon as its context is done, mimicking a real driver.
type ctxPinger struct{}

func (c *ctxPinger) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
