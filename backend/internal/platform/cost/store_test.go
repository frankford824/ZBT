package cost

import "testing"

func TestNormalizeProjectStatusDefaultsOnlyBlankStatus(t *testing.T) {
	status, err := normalizeProjectStatus(" ")
	if err != nil {
		t.Fatalf("expected blank status to default: %v", err)
	}
	if status != "draft" {
		t.Fatalf("expected blank status to default to draft, got %q", status)
	}

	status, err = normalizeProjectStatus(" ACTIVE ")
	if err != nil {
		t.Fatalf("expected known status to normalize: %v", err)
	}
	if status != "active" {
		t.Fatalf("expected known status to normalize to active, got %q", status)
	}

	if _, err := normalizeProjectStatus("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported cost project status to be rejected, got %v", err)
	}
}
