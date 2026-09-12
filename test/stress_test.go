package test

import "testing"

// Reuse the measured workload: actual admission, four players per room,
// continuous output consumption and input, and asserted gameplay delivery.
func TestE2E_StressTestMultipleRooms(t *testing.T) {
	if testing.Short() {
		t.Skip("multiroom load test")
	}
	t.Setenv("PONGO_CLIENTS", "200")
	t.Setenv("PONGO_IDLE", "0")
	runPerformanceRooms(t)
}
