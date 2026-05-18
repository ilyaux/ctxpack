package domain

import "testing"

func TestCalculateCommission(t *testing.T) {
	got := CalculateCommission(10000, "retail")
	if got != 100 {
		t.Fatalf("commission = %d", got)
	}
}
