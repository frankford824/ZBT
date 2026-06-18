package cost

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/frankford824/ZBT/backend/internal/platform/config"
)

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

func TestCostProjectWriteRejectsOversizedNameBeforeDB(t *testing.T) {
	store := NewStore(config.Config{}, nil)
	projectID := "00000000-0000-4000-8000-000000000001"
	oversized := strings.Repeat("成", maxCostNameRunes+1)

	_, err := store.CreateProject(context.Background(), "tenant-id", CreateProjectRequest{
		ProjectID: projectID,
		Name:      oversized,
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized create project name to be rejected before DB, got %v", err)
	}

	_, err = store.UpdateProject(context.Background(), "tenant-id", projectID, UpdateProjectRequest{
		Name: &oversized,
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized update project name to be rejected before DB, got %v", err)
	}
}

func TestBoundedCostTextTrimsGeneratedFallbackNames(t *testing.T) {
	raw := strings.Repeat("项", maxCostNameRunes+10)
	got := boundedCostText(raw, maxCostNameRunes)

	if len([]rune(got)) != maxCostNameRunes {
		t.Fatalf("expected generated fallback name to be capped at %d runes, got %d", maxCostNameRunes, len([]rune(got)))
	}
}

func TestProjectBudgetRejectsInvalidAmountsBeforeDB(t *testing.T) {
	store := NewStore(config.Config{}, nil)
	projectID := "00000000-0000-4000-8000-000000000001"
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), maxCostAmount + 0.01} {
		_, err := store.CreateProject(context.Background(), "tenant-id", CreateProjectRequest{
			ProjectID:    projectID,
			Name:         "成本项目",
			BudgetAmount: &amount,
		})
		if err != ErrInvalidRequest {
			t.Fatalf("expected create project budget %v to be rejected before DB, got %v", amount, err)
		}

		_, err = store.UpdateProject(context.Background(), "tenant-id", projectID, UpdateProjectRequest{
			BudgetAmount: &amount,
		})
		if err != ErrInvalidRequest {
			t.Fatalf("expected update project budget %v to be rejected before DB, got %v", amount, err)
		}
	}
}

func TestNormalizeItemRequestRejectsInvalidAmounts(t *testing.T) {
	for _, req := range []CreateItemRequest{
		{Name: "实施顾问", BudgetAmount: -1},
		{Name: "实施顾问", ActualAmount: -1},
		{Name: "实施顾问", BudgetAmount: math.NaN()},
		{Name: "实施顾问", ActualAmount: math.Inf(1)},
		{Name: "实施顾问", BudgetAmount: maxCostAmount + 0.01},
		{Name: "实施顾问", ActualAmount: maxCostAmount + 0.01},
	} {
		if _, err := normalizeItemRequest(req); err != ErrInvalidRequest {
			t.Fatalf("expected invalid item amount in %+v to be rejected, got %v", req, err)
		}
	}
}

func TestNormalizeItemRequestRejectsOversizedTextFields(t *testing.T) {
	for _, req := range []CreateItemRequest{
		{Name: "实施顾问", Category: strings.Repeat("分", maxCostShortTextRunes+1)},
		{Name: strings.Repeat("顾", maxCostNameRunes+1)},
		{Name: "实施顾问", Vendor: strings.Repeat("供", maxCostNameRunes+1)},
		{Name: "实施顾问", Note: strings.Repeat("备", maxCostNoteRunes+1)},
	} {
		if _, err := normalizeItemRequest(req); err != ErrInvalidRequest {
			t.Fatalf("expected oversized item text in %+v to be rejected, got %v", req, err)
		}
	}
}

func TestNormalizeItemRequestPreservesZeroAmounts(t *testing.T) {
	normalized, err := normalizeItemRequest(CreateItemRequest{Name: "实施顾问"})
	if err != nil {
		t.Fatalf("expected zero amounts to normalize: %v", err)
	}
	if normalized.BudgetAmount != 0 || normalized.ActualAmount != 0 {
		t.Fatalf("expected zero amounts to be preserved, got %+v", normalized)
	}
}

func TestNormalizeItemRequestAcceptsBoundedUnicodeText(t *testing.T) {
	normalized, err := normalizeItemRequest(CreateItemRequest{
		Category: strings.Repeat("分", maxCostShortTextRunes),
		Name:     strings.Repeat("项", maxCostNameRunes),
		Vendor:   strings.Repeat("供", maxCostNameRunes),
		Note:     strings.Repeat("备", maxCostNoteRunes),
	})
	if err != nil {
		t.Fatalf("expected bounded unicode item text to normalize: %v", err)
	}
	if len([]rune(normalized.Category)) != maxCostShortTextRunes ||
		len([]rune(normalized.Name)) != maxCostNameRunes ||
		len([]rune(normalized.Vendor)) != maxCostNameRunes ||
		len([]rune(normalized.Note)) != maxCostNoteRunes {
		t.Fatalf("expected bounded unicode text to be preserved, got %+v", normalized)
	}
}

func TestCostAdviceCallbackRejectsInvalidResultBeforeDB(t *testing.T) {
	store := NewStore(config.Config{}, nil)

	_, err := store.ApplyAdviceCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-cost-advice-00000000-0000-4000-8000-000000000001",
		Status:   "done",
		Result: map[string]any{
			"invalid": math.NaN(),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected invalid callback result JSON to be rejected before DB, got %v", err)
	}
}

func TestCostAdviceCallbackRejectsOversizedResultBeforeDB(t *testing.T) {
	store := NewStore(config.Config{}, nil)

	_, err := store.ApplyAdviceCallback(context.Background(), CallbackPayload{
		TenantID: "tenant-id",
		TaskID:   "task-cost-advice-00000000-0000-4000-8000-000000000001",
		Status:   "done",
		Result: map[string]any{
			"payload": strings.Repeat("结", maxCostTaskResultJSONBytes),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized callback result to be rejected before DB, got %v", err)
	}
}

func TestNormalizeCostAdviceCallbackBoundsErrorMessage(t *testing.T) {
	_, _, err := normalizeCostAdviceCallbackPayload(CallbackPayload{
		TenantID:     "tenant-id",
		TaskID:       "task-cost-advice-00000000-0000-4000-8000-000000000001",
		Status:       "failed",
		ErrorMessage: strings.Repeat("错", maxCostTaskErrorMessageRunes+1),
		Result:       map[string]any{},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized callback error message to be rejected, got %v", err)
	}
}

func TestNormalizeAcceptedTaskRejectsOversizedRoute(t *testing.T) {
	_, _, err := normalizeAcceptedTask(aiTaskAccepted{
		TaskID: "task-cost-advice-00000000-0000-4000-8000-000000000001",
		Status: "running",
		Route: map[string]any{
			"payload": strings.Repeat("路", maxCostTaskRouteJSONBytes),
		},
	})
	if err != ErrInvalidRequest {
		t.Fatalf("expected oversized accepted route to be rejected, got %v", err)
	}
}
