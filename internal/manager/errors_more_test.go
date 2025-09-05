package manager

import "testing"

func TestIsBudgetExceeded(t *testing.T) {
	err := ErrBudgetExceeded("capacity")
	if !IsBudgetExceeded(err) {
		t.Fatalf("expected IsBudgetExceeded true")
	}
}
