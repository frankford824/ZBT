package factualreview

import (
	"testing"
	"time"
)

func TestEvidenceIsBoundToCurrentBodyAndSource(t *testing.T) {
	revision := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	valid := func() map[string]any {
		return map[string]any{
			"status": "pass", "content_sha256": textHash("技术方案40分。"), "source_revision": revision.Format(time.RFC3339Nano),
			"paragraphs": []any{map[string]any{"status": "supported"}},
		}
	}
	if reason := AuditProblem("技术方案 40分。", valid(), revision); reason != "" {
		t.Fatal(reason)
	}
	for _, test := range []string{"changed_body", "changed_source", "missing_audit", "failed_audit", "missing_paragraphs"} {
		t.Run(test, func(t *testing.T) {
			audit := valid()
			text := "技术方案40分。"
			source := revision
			switch test {
			case "changed_body":
				text = "技术方案30分。"
			case "changed_source":
				source = source.Add(time.Second)
			case "missing_audit":
				audit = nil
			case "failed_audit":
				audit["status"] = "needs_review"
			case "missing_paragraphs":
				audit["paragraphs"] = []any{}
			}
			if AuditProblem(text, audit, source) == "" {
				t.Fatal("unsafe audit accepted")
			}
		})
	}
}
