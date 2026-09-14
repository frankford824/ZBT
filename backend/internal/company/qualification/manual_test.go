package qualification

import "testing"

func TestCreateTextRequiresName(t *testing.T) {
	if _, err := createText("   ", true); err != ErrInvalidRequest {
		t.Fatalf("expected empty required text to be rejected, got %v", err)
	}
	if value, err := createText("  建筑业企业资质  ", true); err != nil || value != "建筑业企业资质" {
		t.Fatalf("unexpected normalized value %q, err=%v", value, err)
	}
}

func TestCreateDateAllowsEmptyAndRejectsInvalid(t *testing.T) {
	if value, err := createDate(""); err != nil || value != nil {
		t.Fatalf("expected empty date to become nil, value=%v err=%v", value, err)
	}
	if _, err := createDate("2026/09/14"); err != ErrInvalidRequest {
		t.Fatalf("expected invalid date to be rejected, got %v", err)
	}
}
