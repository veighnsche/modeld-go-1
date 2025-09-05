package httpapi

import "testing"

func TestSetMaxBodyBytes_DefaultWhenNonPositive(t *testing.T) {
	SetMaxBodyBytes(-1)
	if maxBodyBytes != 1<<20 {
		t.Fatalf("expected default 1MiB, got %d", maxBodyBytes)
	}
	SetMaxBodyBytes(0)
	if maxBodyBytes != 1<<20 {
		t.Fatalf("expected default 1MiB on zero, got %d", maxBodyBytes)
	}
}

func TestSetMaxBodyBytes_PositiveSetsValue(t *testing.T) {
	SetMaxBodyBytes(1234)
	if maxBodyBytes != 1234 {
		t.Fatalf("expected 1234, got %d", maxBodyBytes)
	}
}

func TestSetInferTimeoutSeconds_NormalizesNegativeToZero(t *testing.T) {
	SetInferTimeoutSeconds(-5)
	if inferTimeout != 0 {
		t.Fatalf("expected 0, got %d", inferTimeout)
	}
	SetInferTimeoutSeconds(3)
	if inferTimeout != 3 {
		t.Fatalf("expected 3, got %d", inferTimeout)
	}
}
