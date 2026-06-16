package project

import (
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
