package cost

import "testing"

func TestNormalizeProjectStatusDefaultsOnlyBlankStatus(t *testing.T) {
	status, err := normalizeProjectStatus(" ")
	if err != nil {
		t.Fatalf("expected blank status to default: %v", err)
	}
	if status != "draft" {
		t.Fatalf("expected blank status to default to draft, got %q", status)
	}

	status, err = normalizeProjectStatus(" ACTIVE ")
	if err != nil {
		t.Fatalf("expected known status to normalize: %v", err)
	}
	if status != "active" {
		t.Fatalf("expected known status to normalize to active, got %q", status)
	}

	if _, err := normalizeProjectStatus("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported cost project status to be rejected, got %v", err)
	}
}

func TestNormalizeCostTypeDefaultsOnlyBlankType(t *testing.T) {
	costType, err := normalizeCostType(" ")
	if err != nil {
		t.Fatalf("expected blank cost type to default: %v", err)
	}
	if costType != "other" {
		t.Fatalf("expected blank cost type to default to other, got %q", costType)
	}

	costType, err = normalizeCostType(" MATERIAL ")
	if err != nil {
		t.Fatalf("expected known cost type to normalize: %v", err)
	}
	if costType != "material" {
		t.Fatalf("expected known cost type to normalize to material, got %q", costType)
	}

	if _, err := normalizeCostType("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported cost type to be rejected, got %v", err)
	}
}

func TestNormalizeItemStatusDefaultsOnlyBlankStatus(t *testing.T) {
	status, err := normalizeItemStatus(" ")
	if err != nil {
		t.Fatalf("expected blank item status to default: %v", err)
	}
	if status != "planned" {
		t.Fatalf("expected blank item status to default to planned, got %q", status)
	}

	status, err = normalizeItemStatus(" ACTUAL ")
	if err != nil {
		t.Fatalf("expected known item status to normalize: %v", err)
	}
	if status != "actual" {
		t.Fatalf("expected known item status to normalize to actual, got %q", status)
	}

	if _, err := normalizeItemStatus("unknown"); err != ErrInvalidRequest {
		t.Fatalf("expected unsupported item status to be rejected, got %v", err)
	}
}

func TestMergeItemUpdateRequestPreservesOmittedFields(t *testing.T) {
	actualAmount := 88.5
	status := " ACTUAL "
	vendor := ""
	merged, err := mergeItemUpdateRequest(Item{
		Category:     "人力",
		Name:         "实施顾问",
		CostType:     "labor",
		BudgetAmount: 120,
		ActualAmount: 40,
		Status:       "planned",
		Vendor:       "原供应商",
		Note:         "原备注",
	}, UpdateItemRequest{
		ActualAmount: &actualAmount,
		Status:       &status,
		Vendor:       &vendor,
	})
	if err != nil {
		t.Fatalf("expected partial item update to normalize: %v", err)
	}
	if merged.Category != "人力" || merged.Name != "实施顾问" || merged.CostType != "labor" || merged.BudgetAmount != 120 {
		t.Fatalf("expected omitted fields to be preserved, got %+v", merged)
	}
	if merged.ActualAmount != 88.5 || merged.Status != "actual" || merged.Vendor != "" || merged.Note != "原备注" {
		t.Fatalf("expected provided fields to be applied, got %+v", merged)
	}
}

func TestMergeItemUpdateRequestRejectsInvalidProvidedFields(t *testing.T) {
	name := " "
	_, err := mergeItemUpdateRequest(Item{
		Category:     "人力",
		Name:         "实施顾问",
		CostType:     "labor",
		BudgetAmount: 120,
		ActualAmount: 40,
		Status:       "planned",
	}, UpdateItemRequest{Name: &name})
	if err != ErrInvalidRequest {
		t.Fatalf("expected blank provided name to be rejected, got %v", err)
	}
}
