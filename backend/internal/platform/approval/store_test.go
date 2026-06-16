package approval

import "testing"

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
