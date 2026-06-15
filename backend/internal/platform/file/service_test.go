package file

import "testing"

func TestNormalizeOptionalUUIDRejectsInvalidValue(t *testing.T) {
	if _, err := normalizeOptionalUUID("not-a-uuid"); err == nil {
		t.Fatal("expected invalid uuid to be rejected")
	}
}

func TestNormalizeOptionalUUIDTrimsValidValue(t *testing.T) {
	value, err := normalizeOptionalUUID(" 00000000-0000-4000-8000-000000000001 ")
	if err != nil {
		t.Fatalf("expected valid uuid: %v", err)
	}
	if value != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("unexpected normalized uuid: %q", value)
	}
}

func TestSanitizeFilenameKeepsOnlyBaseName(t *testing.T) {
	if got := sanitizeFilename(`..\..\bid.docx`); got != "bid.docx" {
		t.Fatalf("unexpected filename: %q", got)
	}
}
