package project

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
)

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

func TestProjectWriteRejectsOversizedNameBeforeDB(t *testing.T) {
	store := NewStore(nil)
	projectID := "00000000-0000-4000-8000-000000000001"
	oversized := strings.Repeat("项", maxProjectNameRunes+1)

	_, err := store.Create(context.Background(), "tenant-id", "user-id", CreateProjectRequest{Name: oversized})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized create project name to be rejected before DB, got %v", err)
	}

	_, err = store.Update(context.Background(), "tenant-id", "user-id", projectID, UpdateProjectRequest{Name: &oversized})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized update project name to be rejected before DB, got %v", err)
	}
}

func TestNormalizeProjectNameAcceptsBoundedUnicodeText(t *testing.T) {
	bounded := strings.Repeat("项", maxProjectNameRunes)
	got, err := normalizeProjectName(" " + bounded + " ")
	if err != nil {
		t.Fatalf("expected bounded project name to normalize: %v", err)
	}
	if got != bounded {
		t.Fatalf("expected project name to be trimmed and preserved, got %q", got)
	}
}

func TestNormalizeStatusRejectsUnsupportedFilterValues(t *testing.T) {
	status, err := normalizeStatus(" ")
	if err != nil {
		t.Fatalf("expected blank project status to be accepted: %v", err)
	}
	if status != "" {
		t.Fatalf("expected blank project status to stay empty for caller defaulting, got %q", status)
	}

	status, err = normalizeStatus(" SUBMITTED ")
	if err != nil {
		t.Fatalf("expected known project status to normalize: %v", err)
	}
	if status != "submitted" {
		t.Fatalf("expected known project status to normalize to submitted, got %q", status)
	}

	if _, err := normalizeStatus("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported project status to be rejected, got %v", err)
	}
}

func TestNormalizeUpdateStatusEnforcesProjectStateMachine(t *testing.T) {
	if status, err := normalizeUpdateStatus("bidding", nil); err != nil || status != "bidding" {
		t.Fatalf("expected omitted status to preserve current value, got status=%q err=%v", status, err)
	}

	next := " COMPLIANCE_REVIEW "
	status, err := normalizeUpdateStatus("bidding", &next)
	if err != nil {
		t.Fatalf("expected legal transition to normalize: %v", err)
	}
	if status != "compliance_review" {
		t.Fatalf("expected legal transition target to normalize, got %q", status)
	}

	for _, target := range []string{"submitted", "opportunity", " "} {
		target := target
		if _, err := normalizeUpdateStatus("bidding", &target); err != ErrInvalidRequest {
			t.Fatalf("expected update status %q from bidding to be rejected, got %v", target, err)
		}
	}
}

func TestNormalizeResultValueDefaultsOnlyBlankResult(t *testing.T) {
	result, err := normalizeResultValue(" ", "closed")
	if err != nil {
		t.Fatalf("expected blank closed result to default: %v", err)
	}
	if result == nil || *result != "pending" {
		t.Fatalf("expected blank closed result to default to pending, got %v", result)
	}

	result, err = normalizeResultValue(" WON ", "closed")
	if err != nil {
		t.Fatalf("expected known closed result to normalize: %v", err)
	}
	if result == nil || *result != "won" {
		t.Fatalf("expected known closed result to normalize to won, got %v", result)
	}

	if _, err := normalizeResultValue("unknown", "closed"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported closed result to be rejected, got %v", err)
	}

	result, err = normalizeResultValue("won", "submitted")
	if err != nil {
		t.Fatalf("expected non-closed project result to be ignored: %v", err)
	}
	if result != nil {
		t.Fatalf("expected non-closed project result to be nil, got %v", result)
	}
}

func TestNormalizeMilestoneStatusDefaultsOnlyBlankStatus(t *testing.T) {
	status, err := normalizeMilestoneStatus(" ")
	if err != nil {
		t.Fatalf("expected blank milestone status to default: %v", err)
	}
	if status != "pending" {
		t.Fatalf("expected blank milestone status to default to pending, got %q", status)
	}

	status, err = normalizeMilestoneStatus(" DONE ")
	if err != nil {
		t.Fatalf("expected known milestone status to normalize: %v", err)
	}
	if status != "done" {
		t.Fatalf("expected known milestone status to normalize to done, got %q", status)
	}

	if _, err := normalizeMilestoneStatus("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported milestone status to be rejected, got %v", err)
	}
}

func TestNormalizeMilestoneWriteRequestRejectsOversizedText(t *testing.T) {
	for _, req := range []CreateMilestoneRequest{
		{Title: strings.Repeat("里", maxProjectMilestoneTitleRunes+1)},
		{Title: "初验", Note: strings.Repeat("备", maxProjectMilestoneNoteRunes+1)},
	} {
		if _, err := normalizeMilestoneWriteRequest(req); err != ErrInvalidRequest {
			t.Fatalf("expected oversized milestone text in %+v to be rejected, got %v", req, err)
		}
	}
}

func TestNormalizeMilestoneWriteRequestAcceptsBoundedUnicodeText(t *testing.T) {
	normalized, err := normalizeMilestoneWriteRequest(CreateMilestoneRequest{
		Title: " " + strings.Repeat("里", maxProjectMilestoneTitleRunes) + " ",
		Note:  " " + strings.Repeat("备", maxProjectMilestoneNoteRunes) + " ",
	})
	if err != nil {
		t.Fatalf("expected bounded milestone text to normalize: %v", err)
	}
	if len([]rune(normalized.Title)) != maxProjectMilestoneTitleRunes || len([]rune(normalized.Note)) != maxProjectMilestoneNoteRunes {
		t.Fatalf("expected bounded milestone text to be trimmed and preserved, got title=%d note=%d", len([]rune(normalized.Title)), len([]rune(normalized.Note)))
	}
}

func TestBoundedProjectTextTrimsGeneratedFallbackNames(t *testing.T) {
	raw := " " + strings.Repeat("项", maxProjectGeneratedFilenameRunes+10) + " "
	got := boundedProjectText(raw, maxProjectGeneratedFilenameRunes)

	if len([]rune(got)) != maxProjectGeneratedFilenameRunes {
		t.Fatalf("expected generated project text to be capped at %d runes, got %d", maxProjectGeneratedFilenameRunes, len([]rune(got)))
	}
}

func TestMarshalProjectMetadataJSONRejectsInvalidAndOversizedValues(t *testing.T) {
	for name, value := range map[string]map[string]any{
		"invalid number": {"bad": math.NaN()},
		"unsupported":    {"bad": func() {}},
		"oversized":      {"payload": strings.Repeat("案", maxProjectKnowledgeMetadataBytes)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := marshalProjectMetadataJSON(value, maxProjectKnowledgeMetadataBytes); err != ErrInvalidRequest {
				t.Fatalf("expected invalid project metadata to be rejected, got %v", err)
			}
		})
	}
	if raw, err := marshalProjectMetadataJSON(nil, maxProjectKnowledgeMetadataBytes); err != nil || string(raw) != "{}" {
		t.Fatalf("expected nil project metadata to normalize to empty JSON, raw=%q err=%v", raw, err)
	}
}

func TestWonCaseDocumentMetadataCopiesAndOverridesSystemFields(t *testing.T) {
	original := map[string]any{
		"source_project_id": "old-project",
		"source_file_id":    "old-file",
		"custom":            "keep",
	}
	metadata := wonCaseDocumentMetadata(original, "project-1", "file-1")

	if metadata["source_project_id"] != "project-1" || metadata["source_file_id"] != "file-1" {
		t.Fatalf("expected system fields to be overridden, got %#v", metadata)
	}
	if metadata["custom"] != "keep" {
		t.Fatalf("expected custom metadata to be preserved, got %#v", metadata)
	}
	if original["source_project_id"] != "old-project" || original["source_file_id"] != "old-file" {
		t.Fatalf("expected draft metadata to remain unchanged, got %#v", original)
	}
}

func TestMergeMilestoneUpdateRequestPreservesOmittedFields(t *testing.T) {
	dueDate := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	status := " DONE "
	note := ""
	merged := mergeMilestoneUpdateRequest(Milestone{
		Title:     "初验",
		Status:    "pending",
		DueDate:   &dueDate,
		SortOrder: 30,
		Note:      "原备注",
	}, UpdateMilestoneRequest{
		Status: &status,
		Note:   &note,
	})
	if merged.Title != "初验" || merged.DueDate != "2026-09-01" || merged.SortOrder != 30 {
		t.Fatalf("expected omitted milestone fields to be preserved, got %+v", merged)
	}
	normalizedStatus, err := normalizeMilestoneStatus(merged.Status)
	if err != nil {
		t.Fatalf("expected merged milestone status to normalize: %v", err)
	}
	if normalizedStatus != "done" || merged.Note != "" {
		t.Fatalf("expected provided milestone fields to be applied, got %+v", merged)
	}
}

func TestMergeMilestoneUpdateRequestRejectsBlankProvidedTitle(t *testing.T) {
	title := " "
	merged := mergeMilestoneUpdateRequest(Milestone{
		Title:  "初验",
		Status: "pending",
	}, UpdateMilestoneRequest{Title: &title})
	if strings.TrimSpace(merged.Title) != "" {
		t.Fatalf("expected provided blank title to win before validation, got %+v", merged)
	}
}

func TestNormalizeProjectMemberRoleDefaultsAndNormalizes(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: " ", want: "member"},
		{input: " OWNER ", want: "owner"},
		{input: "member", want: "member"},
		{input: "Viewer", want: "viewer"},
	} {
		got, err := normalizeProjectMemberRole(tc.input)
		if err != nil {
			t.Fatalf("expected role %q to normalize: %v", tc.input, err)
		}
		if got != tc.want {
			t.Fatalf("expected role %q to normalize to %q, got %q", tc.input, tc.want, got)
		}
	}
}

func TestNormalizeProjectMemberRoleRejectsUnsupportedValues(t *testing.T) {
	if _, err := normalizeProjectMemberRole("admin"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported project member role to be rejected, got %v", err)
	}
}

func TestEnsureProjectMemberRemovalAllowedKeepsLastOwner(t *testing.T) {
	if err := ensureProjectMemberRemovalAllowed("owner", 0); err != ErrInvalidRequest {
		t.Fatalf("expected last owner removal to be rejected, got %v", err)
	}
	if err := ensureProjectMemberRemovalAllowed("owner", 1); err != nil {
		t.Fatalf("expected owner removal with another owner to pass: %v", err)
	}
	if err := ensureProjectMemberRemovalAllowed("member", 0); err != nil {
		t.Fatalf("expected non-owner removal to pass: %v", err)
	}
}
