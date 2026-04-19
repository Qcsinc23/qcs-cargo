//go:build integration

package api_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	"github.com/Qcsinc23/qcs-cargo/internal/testdata"
	"github.com/stretchr/testify/require"
)

// TestWarehouseBaysMove_OptimisticConcurrency is the Pass 2.5 HIGH-04
// regression test. Two simultaneous bay-move requests with the same
// expected previous_bay must result in exactly one 200 (the winner) and
// one 409 (the loser). The previous blind UPDATE WHERE id = ? would
// have allowed both to succeed and silently split state across bays.
func TestWarehouseBaysMove_OptimisticConcurrency(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	pkgID := testdata.PkgAliceStored1

	// Force a known starting bay so the test is deterministic
	// regardless of seed defaults.
	_, err := db.DB().Exec(
		`UPDATE locker_packages SET storage_bay = 'A1' WHERE id = ?`,
		pkgID,
	)
	require.NoError(t, err)

	// Two concurrent moves both claim the package is currently in A1.
	// Exactly one must win.
	payloadA := []byte(fmt.Sprintf(`{"package_ids":["%s"],"to_bay":"B2","previous_bay":"A1"}`, pkgID))
	payloadB := []byte(fmt.Sprintf(`{"package_ids":["%s"],"to_bay":"C3","previous_bay":"A1"}`, pkgID))
	payloads := [][]byte{payloadA, payloadB}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var ok int
	var conflict int
	var other int
	statuses := make([]int, 0, len(payloads))

	for _, p := range payloads {
		wg.Add(1)
		go func(payload []byte) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/bays/move", bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+staffToken)
			req.Header.Set("Content-Type", "application/json")
			// Generous timeout: the SQLite driver serialises writes,
			// so the second goroutine may briefly block on the busy
			// timeout before its UPDATE returns 0 rows.
			resp, err := app.Test(req, 5000)
			if err != nil {
				t.Errorf("request: %v", err)
				return
			}
			defer resp.Body.Close()
			mu.Lock()
			defer mu.Unlock()
			statuses = append(statuses, resp.StatusCode)
			switch resp.StatusCode {
			case http.StatusOK:
				ok++
			case http.StatusConflict:
				conflict++
			default:
				other++
			}
		}(p)
	}
	wg.Wait()

	if ok != 1 || conflict != 1 || other != 0 {
		t.Fatalf("expected exactly one 200 and one 409, got ok=%d conflict=%d other=%d statuses=%v", ok, conflict, other, statuses)
	}

	// The winning move must have actually persisted: the package is
	// now in either B2 or C3, not still in A1.
	var finalBay string
	err = db.DB().QueryRow(
		`SELECT COALESCE(storage_bay, '') FROM locker_packages WHERE id = ?`,
		pkgID,
	).Scan(&finalBay)
	require.NoError(t, err)
	if finalBay != "B2" && finalBay != "C3" {
		t.Fatalf("expected winner to have moved package to B2 or C3, got %q", finalBay)
	}
}

// TestWarehouseBaysMove_RejectsMissingPreviousBay guards the new
// validation: the wire format requires `previous_bay`. A legacy client
// posting only {package_ids, to_bay} (or the old {bay_id} key) must be
// rejected with 400 so the staff JS update is enforced.
func TestWarehouseBaysMove_RejectsMissingPreviousBay(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	payload := []byte(fmt.Sprintf(`{"package_ids":["%s"],"to_bay":"B2"}`, testdata.PkgAliceStored1))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/warehouse/bays/move", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing previous_bay, got %d", resp.StatusCode)
	}
}

// TestWarehouseServiceQueueUpdate_RejectsArbitraryStatus is the Pass
// 2.5 MED-13 regression test. The handler must reject statuses that
// are not in the lifecycle allowlist (pending|in_progress|completed|cancelled).
func TestWarehouseServiceQueueUpdate_RejectsArbitraryStatus(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	payload := []byte(`{"status":"totally-bogus"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/warehouse/service-queue/sreq_photo_001", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for disallowed status, got %d", resp.StatusCode)
	}
}
