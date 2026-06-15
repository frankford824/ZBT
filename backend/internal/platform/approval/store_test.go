package approval

import "testing"

func TestNormalizeInstanceStatusFilterRejectsUnsupportedValues(t *testing.T) {
	status, err := normalizeInstanceStatusFilter(" ")
	if err != nil {
		t.Fatalf("expected blank approval status filter to be accepted: %v", err)
	}
	if status != "" {
		t.Fatalf("expected blank approval status filter to stay empty, got %q", status)
	}

	status, err = normalizeInstanceStatusFilter(" APPROVED ")
	if err != nil {
		t.Fatalf("expected known approval status filter to normalize: %v", err)
	}
	if status != "approved" {
		t.Fatalf("expected known approval status filter to normalize to approved, got %q", status)
	}

	if _, err := normalizeInstanceStatusFilter("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported approval status filter to be rejected, got %v", err)
	}
}
