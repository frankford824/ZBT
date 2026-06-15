package project

import "testing"

func TestNormalizeCreateStatusDefaultsOnlyBlankStatus(t *testing.T) {
	status, err := normalizeCreateStatus(" ")
	if err != nil {
		t.Fatalf("expected blank status to default: %v", err)
	}
	if status != "opportunity" {
		t.Fatalf("expected blank status to default to opportunity, got %q", status)
	}

	status, err = normalizeCreateStatus(" BIDDING ")
	if err != nil {
		t.Fatalf("expected known status to normalize: %v", err)
	}
	if status != "bidding" {
		t.Fatalf("expected known status to normalize to bidding, got %q", status)
	}

	if _, err := normalizeCreateStatus("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported create status to be rejected, got %v", err)
	}
}
