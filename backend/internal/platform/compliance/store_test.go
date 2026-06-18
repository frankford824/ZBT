package compliance

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"
)

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

func TestNormalizeLevelsDedupesAndBoundsSelections(t *testing.T) {
	levels, err := normalizeLevels([]string{" l2 ", "L2", "L4", ""})
	if err != nil {
		t.Fatalf("expected duplicate known levels to normalize: %v", err)
	}
	if joined := levelsKey(levels); joined != "L2,L4" {
		t.Fatalf("expected duplicate levels to be deduped, got %q", joined)
	}

	if _, err := normalizeLevels([]string{"L1", "L2", "L3", "L4", "L1"}); err != ErrInvalidRequest {
		t.Fatalf("expected oversized level selection list to be rejected, got %v", err)
	}
}

func TestCreateCheckRejectsOversizedNameAndLevelsBeforeDB(t *testing.T) {
	store := NewStore(nil)

	_, err := store.CreateCheck(context.Background(), "tenant-id", CreateCheckRequest{
		Name: strings.Repeat("检", maxComplianceCheckNameRunes+1),
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized check name to be rejected before DB, got %v", err)
	}

	_, err = store.CreateCheck(context.Background(), "tenant-id", CreateCheckRequest{
		Levels: []string{"L1", "L2", "L3", "L4", "L1"},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized level list to be rejected before DB, got %v", err)
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

func TestNormalizeRuleRejectsOversizedTextAndMetadata(t *testing.T) {
	for _, req := range []CreateRuleRequest{
		{Code: strings.Repeat("c", maxComplianceRuleCodeRunes+1), Name: "规则", Category: "分类"},
		{Code: "rule_code", Name: strings.Repeat("规", maxComplianceRuleNameRunes+1), Category: "分类"},
		{Code: "rule_code", Name: "规则", Category: strings.Repeat("分", maxComplianceRuleCategoryRunes+1)},
		{Code: "rule_code", Name: "规则", Category: "分类", Description: strings.Repeat("描", maxComplianceRuleDescriptionRunes+1)},
		{Code: "rule_code", Name: "规则", Category: "分类", Metadata: map[string]any{strings.Repeat("键", maxComplianceRuleMetadataKeyRunes+1): "value"}},
		{Code: "rule_code", Name: "规则", Category: "分类", Metadata: map[string]any{"payload": strings.Repeat("值", maxComplianceRuleMetadataBytes)}},
	} {
		if _, err := normalizeRule(req); err != ErrInvalidRequest {
			t.Fatalf("expected oversized rule request in %+v to be rejected, got %v", req, err)
		}
	}
}

func TestNormalizeRuleTrimsMetadataKeysAndAcceptsBoundedUnicodeText(t *testing.T) {
	rule, err := normalizeRule(CreateRuleRequest{
		Code:        strings.Repeat("c", maxComplianceRuleCodeRunes),
		Name:        strings.Repeat("规", maxComplianceRuleNameRunes),
		Category:    strings.Repeat("分", maxComplianceRuleCategoryRunes),
		Description: strings.Repeat("描", maxComplianceRuleDescriptionRunes),
		Metadata:    map[string]any{" 来源 ": "manual"},
	})
	if err != nil {
		t.Fatalf("expected bounded rule request to normalize: %v", err)
	}
	if _, ok := rule.Metadata["来源"]; !ok {
		t.Fatalf("expected metadata key to be trimmed, got %+v", rule.Metadata)
	}
}

func TestNormalizeRuleMetadataRejectsTooManyEntries(t *testing.T) {
	metadata := map[string]any{}
	for index := 0; index <= maxComplianceRuleMetadataEntries; index++ {
		metadata["key"+itoa(index)] = "value"
	}
	if _, err := normalizeRuleMetadata(metadata); err != ErrInvalidRequest {
		t.Fatalf("expected oversized metadata entry count to be rejected, got %v", err)
	}
}

func TestMarshalComplianceJSONRejectsInvalidAndOversizedValues(t *testing.T) {
	for name, value := range map[string]any{
		"invalid number": map[string]any{"bad": math.NaN()},
		"unsupported":    map[string]any{"bad": func() {}},
		"oversized":      map[string]any{"payload": strings.Repeat("值", maxComplianceReportMetadataBytes)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := marshalComplianceJSON(value, maxComplianceReportMetadataBytes); err != ErrInvalidRequest {
				t.Fatalf("expected invalid compliance JSON to be rejected, got %v", err)
			}
		})
	}
}

func TestUnmarshalComplianceJSONRejectsInvalidStoredFields(t *testing.T) {
	for name, raw := range map[string][]byte{
		"invalid syntax": []byte(`{"config":`),
		"invalid shape":  []byte(`[{"config":true}]`),
		"oversized":      []byte(`{"payload":"` + strings.Repeat("值", maxComplianceCheckConfigBytes) + `"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := unmarshalComplianceJSON(raw, maxComplianceCheckConfigBytes); err == nil {
				t.Fatal("expected invalid stored compliance JSON to be rejected")
			}
		})
	}
}

func TestUnmarshalComplianceJSONNormalizesEmptyStoredFields(t *testing.T) {
	for name, raw := range map[string][]byte{
		"nil":   nil,
		"blank": []byte("   "),
		"null":  []byte(" null "),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := unmarshalComplianceJSON(raw, maxComplianceCheckConfigBytes)
			if err != nil {
				t.Fatalf("expected empty stored compliance JSON to normalize: %v", err)
			}
			if got == nil || len(got) != 0 {
				t.Fatalf("expected empty map, got %+v", got)
			}
		})
	}
}

func TestScanComplianceRowsRejectInvalidStoredJSON(t *testing.T) {
	if _, err := scanCheck(complianceCheckScanRow{configRaw: []byte(`{"levels":`)}); err == nil {
		t.Fatal("expected invalid check config JSON to be rejected")
	}
	if _, err := scanIssue(complianceIssueScanRow{locationRaw: []byte(`[{"page":1}]`)}); err == nil {
		t.Fatal("expected invalid issue location JSON shape to be rejected")
	}
	rawRuleMetadata := []byte(`{"` + strings.Repeat("键", maxComplianceRuleMetadataKeyRunes+1) + `":"value"}`)
	if _, err := scanRule(complianceRuleScanRow{metadataRaw: rawRuleMetadata}); err != ErrInvalidRequest {
		t.Fatalf("expected invalid rule metadata to be rejected, got %v", err)
	}
}

func TestScanComplianceRowsNormalizeEmptyStoredJSON(t *testing.T) {
	check, err := scanCheck(complianceCheckScanRow{configRaw: nil})
	if err != nil {
		t.Fatalf("expected empty check config to normalize: %v", err)
	}
	if check.Config == nil || len(check.Config) != 0 {
		t.Fatalf("expected empty check config map, got %+v", check.Config)
	}

	issue, err := scanIssue(complianceIssueScanRow{locationRaw: []byte(" null ")})
	if err != nil {
		t.Fatalf("expected empty issue location to normalize: %v", err)
	}
	if issue.Location == nil || len(issue.Location) != 0 {
		t.Fatalf("expected empty issue location map, got %+v", issue.Location)
	}

	rule, err := scanRule(complianceRuleScanRow{metadataRaw: []byte("{}")})
	if err != nil {
		t.Fatalf("expected empty rule metadata to normalize: %v", err)
	}
	if rule.Metadata == nil || len(rule.Metadata) != 0 {
		t.Fatalf("expected empty rule metadata map, got %+v", rule.Metadata)
	}
}

func TestBoundedComplianceTextTrimsGeneratedIssueText(t *testing.T) {
	raw := " " + strings.Repeat("问", maxComplianceIssueTitleRunes+10) + " "
	got := boundedComplianceText(raw, maxComplianceIssueTitleRunes)

	if len([]rune(got)) != maxComplianceIssueTitleRunes {
		t.Fatalf("expected generated issue text to be capped at %d runes, got %d", maxComplianceIssueTitleRunes, len([]rune(got)))
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

type complianceCheckScanRow struct {
	configRaw []byte
}

func (row complianceCheckScanRow) Scan(dest ...any) error {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	*(dest[0].(*string)) = "check-1"
	*(dest[1].(*sql.NullString)) = sql.NullString{}
	*(dest[2].(*string)) = "标书"
	*(dest[3].(*string)) = "合规检查"
	*(dest[4].(*string)) = "done"
	*(dest[5].(*string)) = "pass"
	*(dest[6].(*int)) = 100
	*(dest[7].(*[]byte)) = row.configRaw
	*(dest[8].(*sql.NullString)) = sql.NullString{}
	*(dest[9].(*int)) = 0
	*(dest[10].(*sql.NullTime)) = sql.NullTime{}
	*(dest[11].(*sql.NullTime)) = sql.NullTime{}
	*(dest[12].(*time.Time)) = now
	*(dest[13].(*time.Time)) = now
	return nil
}

type complianceIssueScanRow struct {
	locationRaw []byte
}

func (row complianceIssueScanRow) Scan(dest ...any) error {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	*(dest[0].(*string)) = "issue-1"
	*(dest[1].(*string)) = "check-1"
	*(dest[2].(*sql.NullString)) = sql.NullString{}
	*(dest[3].(*string)) = "格式规范"
	*(dest[4].(*string)) = "warn"
	*(dest[5].(*string)) = "open"
	*(dest[6].(*string)) = "问题"
	*(dest[7].(*string)) = "证据"
	*(dest[8].(*string)) = "建议"
	*(dest[9].(*[]byte)) = row.locationRaw
	*(dest[10].(*time.Time)) = now
	*(dest[11].(*time.Time)) = now
	return nil
}

type complianceRuleScanRow struct {
	metadataRaw []byte
}

func (row complianceRuleScanRow) Scan(dest ...any) error {
	now := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	*(dest[0].(*string)) = "rule-1"
	*(dest[1].(*string)) = "rule_code"
	*(dest[2].(*string)) = "规则"
	*(dest[3].(*string)) = "分类"
	*(dest[4].(*string)) = "L1"
	*(dest[5].(*string)) = "warn"
	*(dest[6].(*string)) = "说明"
	*(dest[7].(*bool)) = true
	*(dest[8].(*[]byte)) = row.metadataRaw
	*(dest[9].(*time.Time)) = now
	*(dest[10].(*time.Time)) = now
	return nil
}
