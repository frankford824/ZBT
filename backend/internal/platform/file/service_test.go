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

func TestSanitizeFilenameRemovesControlCharacters(t *testing.T) {
	got := sanitizeFilename("..\\bad\r\nX-Injected: yes.pdf")
	if got != "badX-Injected: yes.pdf" {
		t.Fatalf("unexpected sanitized filename: %q", got)
	}
}

func TestContentDispositionUsesHeaderSafeFallbackName(t *testing.T) {
	header := contentDisposition("attachment", "投标\"\r\n文件\\demo.pdf")
	if header != `attachment; filename="投标文件demo.pdf"; filename*=UTF-8''%E6%8A%95%E6%A0%87%22%E6%96%87%E4%BB%B6%5Cdemo.pdf` {
		t.Fatalf("unexpected content disposition: %q", header)
	}
}
