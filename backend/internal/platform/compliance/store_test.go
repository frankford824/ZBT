package compliance

import "testing"

func TestNormalizeLevelsDefaultsOnlyBlankLevels(t *testing.T) {
	levels, err := normalizeLevels(nil)
	if err != nil {
		t.Fatalf("expected missing levels to default: %v", err)
	}
	if joined := levelsKey(levels); joined != "L1,L2,L3" {
		t.Fatalf("expected missing levels to default to L1,L2,L3, got %q", joined)
	}

	levels, err = normalizeLevels([]string{" l2 ", "", "L4"})
	if err != nil {
		t.Fatalf("expected known levels to normalize: %v", err)
	}
	if joined := levelsKey(levels); joined != "L2,L4" {
		t.Fatalf("expected known levels to normalize to L2,L4, got %q", joined)
	}

	if _, err := normalizeLevels([]string{"L1", "bad"}); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported level to be rejected, got %v", err)
	}
}

func TestNormalizeRuleDefaultsOnlyBlankEnums(t *testing.T) {
	rule, err := normalizeRule(CreateRuleRequest{
		Code:     "signature_required",
		Name:     "签章要求",
		Category: "格式规范",
	})
	if err != nil {
		t.Fatalf("expected blank rule enums to default: %v", err)
	}
	if rule.Level != "L1" {
		t.Fatalf("expected blank rule level to default to L1, got %q", rule.Level)
	}
	if rule.Severity != "warn" {
		t.Fatalf("expected blank rule severity to default to warn, got %q", rule.Severity)
	}

	rule, err = normalizeRule(CreateRuleRequest{
		Code:     "validity_days",
		Name:     "投标有效期",
		Category: "商务响应",
		Level:    " l3 ",
		Severity: " FAIL_CANDIDATE ",
	})
	if err != nil {
		t.Fatalf("expected known rule enums to normalize: %v", err)
	}
	if rule.Level != "L3" || rule.Severity != "fail_candidate" {
		t.Fatalf("expected known rule enums to normalize, got level=%q severity=%q", rule.Level, rule.Severity)
	}

	if _, err := normalizeRule(CreateRuleRequest{Code: "x", Name: "x", Category: "x", Level: "bad"}); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported rule level to be rejected, got %v", err)
	}
	if _, err := normalizeRule(CreateRuleRequest{Code: "x", Name: "x", Category: "x", Severity: "bad"}); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported rule severity to be rejected, got %v", err)
	}
}

func TestCompliancePipelineGateStatusMapsResultStatus(t *testing.T) {
	for resultStatus, want := range map[string]string{
		"pass":           "passed",
		"warn":           "needs_review",
		"fail_candidate": "needs_review",
		"fail":           "blocked",
		"unknown":        "",
	} {
		if got := compliancePipelineGateStatus(resultStatus); got != want {
			t.Fatalf("expected result status %q to map to %q, got %q", resultStatus, want, got)
		}
	}
}

func TestNullableUUIDStringOnlyReturnsConcreteUUIDString(t *testing.T) {
	if got := nullableUUIDString("00000000-0000-4000-8000-000000000001"); got == "" {
		t.Fatal("expected string value to be returned")
	}
	if got := nullableUUIDString(nil); got != "" {
		t.Fatalf("expected nil to return empty string, got %q", got)
	}
}

func levelsKey(levels []string) string {
	result := ""
	for index, level := range levels {
		if index > 0 {
			result += ","
		}
		result += level
	}
	return result
}
