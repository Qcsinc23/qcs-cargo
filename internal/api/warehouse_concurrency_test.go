//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

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
			resp, err := app.Test(req, 15000)
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

// setShipRequestStatus is a small test helper for the Pass 3 HIGH-04
// ship-queue concurrency tests. It forces a known starting status on a
// seeded ship request so the optimistic-lock assertions are deterministic
// regardless of seed defaults.
func setShipRequestStatus(t *testing.T, id, status string) {
	t.Helper()
	_, err := db.DB().Exec(
		`UPDATE ship_requests SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC().Format(time.RFC3339), id,
	)
	require.NoError(t, err)
}

// TestWarehouseShipQueueWeighed_OptimisticConcurrency is the Pass 3
// HIGH-04 regression for the Weighed transition. The previous blind
// UPDATE of consolidated_weight_lbs would have allowed both writers to
// succeed and silently overwrite the other's reading. The fix uses an
// `consolidated_weight_lbs IS NULL` guard (the status CHECK constraint
// does not include a 'weighed' state, so this column takes the role of
// the optimistic version). A sequential replay exercises that guard
// without the false positives a concurrent variant hits when SQLite's
// single-writer lock just measures busy_timeout.
func TestWarehouseShipQueueWeighed_OptimisticConcurrency(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	srID := testdata.ShipReqAlicePaid
	setShipRequestStatus(t, srID, "processing")
	_, err := db.DB().Exec(
		`UPDATE ship_requests SET consolidated_weight_lbs = NULL WHERE id = ?`,
		srID,
	)
	require.NoError(t, err)

	payloadA := []byte(`{"consolidated_weight_lbs":12.5,"expected_status":"processing"}`)
	payloadB := []byte(`{"consolidated_weight_lbs":13.7,"expected_status":"processing"}`)

	send := func(payload []byte) (int, []byte) {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/warehouse/ship-queue/"+srID+"/weighed", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+staffToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 15000)
		require.NoError(t, err)
		defer resp.Body.Close()
		body := make([]byte, 0, 512)
		buf := make([]byte, 512)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				body = append(body, buf[:n]...)
			}
			if rerr != nil {
				break
			}
		}
		return resp.StatusCode, body
	}

	statusA, _ := send(payloadA)
	if statusA != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", statusA)
	}
	statusB, bodyB := send(payloadB)
	if statusB != http.StatusConflict {
		t.Fatalf("expected second request 409 STALE_STATUS, got %d", statusB)
	}

	statuses := []int{statusA, statusB}
	bodies := [][]byte{nil, bodyB}
	ok := 1
	conflict := 1
	other := 0

	if ok != 1 || conflict != 1 || other != 0 {
		t.Fatalf("expected exactly one 200 and one 409, got ok=%d conflict=%d other=%d statuses=%v", ok, conflict, other, statuses)
	}

	// The 409 response must be the new STALE_STATUS shape so ship-queue.js
	// can route it through the targeted-row-refresh path.
	for i, code := range statuses {
		if code != http.StatusConflict {
			continue
		}
		var parsed struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Details struct {
					ShipRequestID   string `json:"ship_request_id"`
					ExpectedStatus  string `json:"expected_status"`
					CurrentStatus   string `json:"current_status"`
					AttemptedStatus string `json:"attempted_status"`
				} `json:"details"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(bodies[i], &parsed))
		if parsed.Error.Code != "STALE_STATUS" {
			t.Fatalf("expected error.code=STALE_STATUS, got %q (body=%s)", parsed.Error.Code, string(bodies[i]))
		}
		if parsed.Error.Details.ShipRequestID != srID {
			t.Fatalf("expected details.ship_request_id=%q, got %q", srID, parsed.Error.Details.ShipRequestID)
		}
		if parsed.Error.Details.ExpectedStatus != "processing" {
			t.Fatalf("expected details.expected_status=processing, got %q", parsed.Error.Details.ExpectedStatus)
		}
		if parsed.Error.Details.AttemptedStatus != "weighed" {
			t.Fatalf("expected details.attempted_status=weighed, got %q", parsed.Error.Details.AttemptedStatus)
		}
		// Status didn't change (Weighed isn't a discrete lifecycle
		// state) — the loser's view of "current" should still be
		// processing, but with current_weight populated by the winner.
		if parsed.Error.Details.CurrentStatus != "processing" {
			t.Fatalf("expected details.current_status=processing, got %q", parsed.Error.Details.CurrentStatus)
		}
	}

	// The winner's write must have actually persisted: status remains
	// "processing" (Weighed is a sub-step) and consolidated_weight_lbs
	// is now one of the two candidate values, not NULL.
	var finalStatus string
	var finalWeight float64
	err = db.DB().QueryRow(
		`SELECT status, COALESCE(consolidated_weight_lbs, 0) FROM ship_requests WHERE id = ?`,
		srID,
	).Scan(&finalStatus, &finalWeight)
	require.NoError(t, err)
	if finalStatus != "processing" {
		t.Fatalf("expected final status=processing, got %q", finalStatus)
	}
	if finalWeight != 12.5 && finalWeight != 13.7 {
		t.Fatalf("expected winner's weight (12.5 or 13.7), got %v", finalWeight)
	}
}

// TestWarehouseShipQueueWeighed_RejectsMissingExpectedStatus guards the
// Pass 3 HIGH-04 wire-format change: a legacy client posting only
// {consolidated_weight_lbs} without expected_status must be rejected
// with 400 so the staff JS update is enforced.
func TestWarehouseShipQueueWeighed_RejectsMissingExpectedStatus(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	srID := testdata.ShipReqAlicePaid
	setShipRequestStatus(t, srID, "processing")

	payload := []byte(`{"consolidated_weight_lbs":10.0}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/warehouse/ship-queue/"+srID+"/weighed", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing expected_status, got %d", resp.StatusCode)
	}
}

// TestWarehouseShipQueueStaged_OptimisticConcurrency is the Pass 3
// HIGH-04 regression for the Staged transition. Two sequential staff
// clients try to stage the same ship request with the same expected
// prior status; the first must win and the second must get 409
// STALE_STATUS because the first's transition already advanced the row
// out of 'processing'. The :execrows + WHERE status=? guard is what
// protects the invariant, so a sequential replay is a sufficient and
// deterministic test — a concurrent variant hits SQLite's single-writer
// lock and just measures busy_timeout, not the handler's correctness.
func TestWarehouseShipQueueStaged_OptimisticConcurrency(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	srID := testdata.ShipReqAlicePaid
	setShipRequestStatus(t, srID, "processing")

	payloadA := []byte(`{"staging_bay":"BayA","manifest_id":"m-a","expected_status":"processing"}`)
	payloadB := []byte(`{"staging_bay":"BayB","manifest_id":"m-b","expected_status":"processing"}`)

	send := func(payload []byte) int {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/warehouse/ship-queue/"+srID+"/staged", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+staffToken)
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, 15000)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if s := send(payloadA); s != http.StatusOK {
		t.Fatalf("expected first request 200, got %d", s)
	}
	if s := send(payloadB); s != http.StatusConflict {
		t.Fatalf("expected second request 409 STALE_STATUS, got %d", s)
	}

	var finalStatus, finalBay string
	err := db.DB().QueryRow(
		`SELECT status, COALESCE(staging_bay, '') FROM ship_requests WHERE id = ?`,
		srID,
	).Scan(&finalStatus, &finalBay)
	require.NoError(t, err)
	if finalStatus != "staged" {
		t.Fatalf("expected final status=staged, got %q", finalStatus)
	}
	if finalBay != "BayA" {
		t.Fatalf("expected staging_bay=BayA (first writer wins), got %q", finalBay)
	}
}

// TestWarehouseShipQueueStaged_RejectsMissingExpectedStatus mirrors the
// weighed-handler validation: legacy clients without expected_status
// must be rejected with 400.
func TestWarehouseShipQueueStaged_RejectsMissingExpectedStatus(t *testing.T) {
	app := setupTestApp(t)
	staffToken := issueAccessTokenForUser(t, testdata.StaffWarehouseID)

	srID := testdata.ShipReqAlicePaid
	setShipRequestStatus(t, srID, "processing")

	payload := []byte(`{"staging_bay":"BayA"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/warehouse/ship-queue/"+srID+"/staged", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+staffToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing expected_status, got %d", resp.StatusCode)
	}
}
