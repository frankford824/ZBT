package approval

import (
	"context"
	"strings"
	"testing"
)

func TestCurrentApprovalStepUsesOneBasedActiveStep(t *testing.T) {
	steps := []Step{
		{Order: 1, Name: "first", RoleCode: "department_admin", Required: true},
		{Order: 2, Name: "second", RoleCode: "project_manager", Required: true},
	}

	step, active, err := currentApprovalStep(steps, 2)
	if err != nil {
		t.Fatalf("expected current step to be resolved: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected two active steps, got %d", len(active))
	}
	if step.Name != "second" {
		t.Fatalf("expected one-based current_step=2 to resolve second step, got %q", step.Name)
	}
}

func TestCurrentApprovalStepRejectsInvalidPosition(t *testing.T) {
	for _, currentStep := range []int{0, 3} {
		if _, _, err := currentApprovalStep([]Step{{Order: 1, Name: "first", Required: true}}, currentStep); err != ErrInvalidRequest {
			t.Fatalf("expected current_step=%d to be rejected, got %v", currentStep, err)
		}
	}
	if _, _, err := currentApprovalStep(nil, 1); err != ErrInvalidRequest {
		t.Fatalf("expected empty snapshot to be rejected, got %v", err)
	}
}

func TestCurrentApprovalStepFiltersRequiredSteps(t *testing.T) {
	steps := []Step{
		{Order: 1, Name: "optional", RoleCode: "company_admin", Required: false},
		{Order: 2, Name: "required", RoleCode: "project_manager", Required: true},
	}

	step, active, err := currentApprovalStep(steps, 1)
	if err != nil {
		t.Fatalf("expected required step to be resolved: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected only required steps to be active, got %d", len(active))
	}
	if step.Name != "required" {
		t.Fatalf("expected first active step to be required step, got %q", step.Name)
	}
}

func TestNormalizeChainRejectsInvalidStepUserID(t *testing.T) {
	_, err := normalizeChain(CreateChainRequest{
		Name:         "chain",
		ResourceType: "bid",
		Steps: []Step{
			{Name: "指定审批", UserID: "not-a-uuid", Required: true},
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid step user id to be rejected, got %v", err)
	}
}

func TestCreateChainRejectsOversizedConfigBeforeDB(t *testing.T) {
	store := NewStore(nil)

	_, err := store.CreateChain(context.Background(), "tenant-id", CreateChainRequest{
		Name: strings.Repeat("审", maxApprovalChainNameRunes+1),
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized chain name to be rejected before DB, got %v", err)
	}

	steps := make([]Step, maxApprovalSteps+1)
	for index := range steps {
		steps[index] = Step{Name: "审批级", RoleCode: "company_admin", Required: true}
	}
	_, err = store.CreateChain(context.Background(), "tenant-id", CreateChainRequest{
		Name:         "审批链",
		ResourceType: "bid",
		Steps:        steps,
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized step list to be rejected before DB, got %v", err)
	}
}

func TestNormalizeChainRejectsOversizedTextFields(t *testing.T) {
	for _, req := range []CreateChainRequest{
		{Name: strings.Repeat("审", maxApprovalChainNameRunes+1), ResourceType: "bid"},
		{Name: "审批链", Description: strings.Repeat("描", maxApprovalChainDescriptionRunes+1), ResourceType: "bid"},
		{Name: "审批链", ResourceType: strings.Repeat("b", maxApprovalResourceTypeRunes+1)},
		{Name: "审批链", ResourceType: "bid", Steps: []Step{{Name: strings.Repeat("级", maxApprovalStepNameRunes+1), RoleCode: "company_admin", Required: true}}},
		{Name: "审批链", ResourceType: "bid", Steps: []Step{{Name: "审批级", RoleCode: strings.Repeat("r", maxApprovalStepRoleCodeRunes+1), Required: true}}},
		{Name: "审批链", ResourceType: "bid", Steps: []Step{{Name: "审批级", RoleCode: "company_admin", Required: true, Condition: strings.Repeat("条", maxApprovalStepConditionRunes+1)}}},
	} {
		if _, err := normalizeChain(req); err != ErrInvalidRequest {
			t.Fatalf("expected oversized chain request in %+v to be rejected, got %v", req, err)
		}
	}
}

func TestNormalizeChainAcceptsBoundedUnicodeText(t *testing.T) {
	chain, err := normalizeChain(CreateChainRequest{
		Name:         " " + strings.Repeat("审", maxApprovalChainNameRunes) + " ",
		Description:  " " + strings.Repeat("描", maxApprovalChainDescriptionRunes) + " ",
		ResourceType: " bid ",
		Steps: []Step{{
			Name:      " " + strings.Repeat("级", maxApprovalStepNameRunes) + " ",
			RoleCode:  " company_admin ",
			Required:  true,
			Condition: " " + strings.Repeat("条", maxApprovalStepConditionRunes) + " ",
		}},
	})
	if err != nil {
		t.Fatalf("expected bounded chain request to normalize: %v", err)
	}
	if len([]rune(chain.Name)) != maxApprovalChainNameRunes || len([]rune(chain.Steps[0].Condition)) != maxApprovalStepConditionRunes {
		t.Fatalf("expected bounded unicode text to be trimmed and preserved, got name=%d condition=%d", len([]rune(chain.Name)), len([]rune(chain.Steps[0].Condition)))
	}
}

func TestNormalizeChainCanonicalizesStepUserID(t *testing.T) {
	chain, err := normalizeChain(CreateChainRequest{
		Name:         "chain",
		ResourceType: "bid",
		Steps: []Step{
			{Name: "指定审批", UserID: " AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA ", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("expected uppercase step user id to normalize: %v", err)
	}
	if chain.Steps[0].UserID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("expected canonical step user id, got %q", chain.Steps[0].UserID)
	}
}

func TestNormalizeDecisionCommentBoundsAndTrims(t *testing.T) {
	comment := strings.Repeat("意", maxApprovalDecisionCommentRunes)
	got, err := normalizeDecisionComment(" " + comment + " ")
	if err != nil {
		t.Fatalf("expected bounded decision comment to normalize: %v", err)
	}
	if got != comment {
		t.Fatalf("expected decision comment to be trimmed and preserved, got %q", got)
	}

	if _, err := normalizeDecisionComment(strings.Repeat("意", maxApprovalDecisionCommentRunes+1)); err != ErrInvalidRequest {
		t.Fatalf("expected oversized decision comment to be rejected, got %v", err)
	}
}

func TestMarshalApprovalStepsRejectsOversizedSnapshot(t *testing.T) {
	steps := make([]Step, maxApprovalSteps+1)
	for index := range steps {
		steps[index] = Step{Name: "审批级", RoleCode: "company_admin", Required: true}
	}
	if _, err := marshalApprovalSteps(steps); err != ErrInvalidRequest {
		t.Fatalf("expected oversized approval snapshot to be rejected, got %v", err)
	}

	if _, err := marshalApprovalSteps([]Step{{Name: strings.Repeat("级", maxApprovalStepNameRunes+1), RoleCode: "company_admin"}}); err != ErrInvalidRequest {
		t.Fatalf("expected oversized approval snapshot text to be rejected, got %v", err)
	}
}

func TestBoundedApprovalTextTrimsGeneratedValues(t *testing.T) {
	raw := " " + strings.Repeat("标", maxApprovalInstanceTitleRunes+10) + " "
	got := boundedApprovalText(raw, maxApprovalInstanceTitleRunes)

	if len([]rune(got)) != maxApprovalInstanceTitleRunes {
		t.Fatalf("expected generated approval text to be capped at %d runes, got %d", maxApprovalInstanceTitleRunes, len([]rune(got)))
	}
}

func TestNormalizeInstanceStatusFilterRejectsUnsupportedValues(t *testing.T) {
	status, err := normalizeInstanceStatusFilter(" ")
	if err != nil {
		t.Fatalf("expected blank approval status filter to be accepted: %v", err)
	}
	if status != "" {
		t.Fatalf("expected blank approval status filter to stay empty, got %q", status)
	}

	status, err = normalizeInstanceStatusFilter(" APPROVED ")
	if err != nil {
		t.Fatalf("expected known approval status filter to normalize: %v", err)
	}
	if status != "approved" {
		t.Fatalf("expected known approval status filter to normalize to approved, got %q", status)
	}

	if _, err := normalizeInstanceStatusFilter("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported approval status filter to be rejected, got %v", err)
	}
}
