package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
)

var errDegraded = errors.New("ping boom")

// newDegradedDetector builds a detector whose pinger dependencies return the
// given errors. It returns the detector plus the backing mocks so tests can
// assert on the pings.
func newDegradedDetector(t *testing.T, dbErr, redisErr error, maintFile string) (*DegradedDetector, *mockcontracts.MockDBPinger, *mockcontracts.MockSessionPinger) {
	t.Helper()

	dbp := mockcontracts.NewMockDBPinger(t)
	dbp.On("Ping", mock.Anything).Return(dbErr)
	sess := mockcontracts.NewMockSessionPinger(t)
	sess.On("Ping", mock.Anything).Return(redisErr)

	// halt keeps the background poll goroutine quiescent so the test drives
	// detect() directly; detect() itself does not consult the halted flag.
	d := &DegradedDetector{
		pool:      dbp,
		sessions:  sess,
		maintFile: maintFile,
	}
	d.Halt()
	return d, dbp, sess
}

func TestDegradedDetector_Healthy(t *testing.T) {
	d, _, _ := newDegradedDetector(t, nil, nil, filepath.Join(t.TempDir(), "missing"))
	d.detect()
	require.False(t, d.IsDegraded(), "expected not degraded when db, redis, and maintenance file are all absent")
}

func TestDegradedDetector_DatabaseDown(t *testing.T) {
	d, _, _ := newDegradedDetector(t, errDegraded, nil, filepath.Join(t.TempDir(), "missing"))
	d.detect()
	require.True(t, d.IsDegraded(), "expected degraded when postgres ping fails")
}

func TestDegradedDetector_RedisDown(t *testing.T) {
	d, _, _ := newDegradedDetector(t, nil, errDegraded, filepath.Join(t.TempDir(), "missing"))
	d.detect()
	require.True(t, d.IsDegraded(), "expected degraded when redis ping fails")
}

func TestDegradedDetector_MaintenanceFilePresent(t *testing.T) {
	maint := filepath.Join(t.TempDir(), "maintenance")
	require.NoError(t, os.WriteFile(maint, []byte("down for maintenance"), 0o644))
	d, _, _ := newDegradedDetector(t, nil, nil, maint)
	d.detect()
	require.True(t, d.IsDegraded(), "expected degraded when maintenance file exists")
}

func TestDegradedDetector_Recovers(t *testing.T) {
	dbp := mockcontracts.NewMockDBPinger(t)
	dbp.On("Ping", mock.Anything).Return(errDegraded).Once()
	dbp.On("Ping", mock.Anything).Return(nil).Once()
	sess := mockcontracts.NewMockSessionPinger(t)
	sess.On("Ping", mock.Anything).Return(errDegraded).Once()
	sess.On("Ping", mock.Anything).Return(nil).Once()
	maint := filepath.Join(t.TempDir(), "maintenance")
	require.NoError(t, os.WriteFile(maint, []byte("x"), 0o644))

	d := &DegradedDetector{pool: dbp, sessions: sess, maintFile: maint}
	d.detect()
	require.True(t, d.IsDegraded(), "expected degraded while deps are down")

	// Dependencies and maintenance both recover.
	require.NoError(t, os.Remove(maint))
	d.detect()
	require.False(t, d.IsDegraded(), "expected to recover once deps and maintenance are cleared")
}

func TestNewDegradedDetector_SetsInitialState(t *testing.T) {
	// No pings happen before the detector is halted, so no expectations are
	// registered.
	d := NewDegradedDetector(mockcontracts.NewMockDBPinger(t), mockcontracts.NewMockSessionPinger(t), filepath.Join(t.TempDir(), "missing"))
	defer d.Halt()
	require.False(t, d.IsDegraded(), "expected to start not degraded")
}

func TestDegradedDetector_WrapViaDetect(t *testing.T) {
	// End-to-end: a failed probe marks the detector degraded, and Wrap then
	// blocks traffic while still serving probes.
	d, _, _ := newDegradedDetector(t, errDegraded, nil, filepath.Join(t.TempDir(), "missing"))
	d.detect()

	rec := httptest.NewRecorder()
	d.Wrap(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/communities", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, `{"error":"service unavailable"}`, rec.Body.String())

	for _, p := range []string{"/healthz", "/readyz", "/metrics", "/"} {
		rec := httptest.NewRecorder()
		d.Wrap(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		require.Equal(t, http.StatusNoContent, rec.Code, "path: %s", p)
	}
}
