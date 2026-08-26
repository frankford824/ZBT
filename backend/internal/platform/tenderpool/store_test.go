package tenderpool

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func validRunInput() CollectorRunInput {
	return CollectorRunInput{
		ExternalSource: "zbcg",
		Status:         "ok",
		FetchedCount:   1,
		StartedAt:      "2026-08-25T13:00:00Z",
	}
}

func validTenderInput() TenderInput {
	return TenderInput{
		ExternalID: "1234567890",
		Title:      "某高速公路桥梁定期检查服务采购",
	}
}

func requestWithTenders(tenders ...TenderInput) IngestRequest {
	return IngestRequest{Run: validRunInput(), Tenders: tenders}
}

func TestNormalizeIngestRequestNormalizesMinimalPayload(t *testing.T) {
	item := validTenderInput()
	item.Title = "  某高速公路桥梁定期检查服务采购  "

	normalized, err := normalizeIngestRequest(requestWithTenders(item))
	if err != nil {
		t.Fatalf("expected minimal ingest payload to normalize: %v", err)
	}
	if normalized.Run.ExternalSource != "zbcg" || normalized.Run.Status != "ok" {
		t.Fatalf("unexpected normalized run: %+v", normalized.Run)
	}
	if !normalized.Run.StartedAt.Equal(time.Date(2026, 8, 25, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected started_at: %s", normalized.Run.StartedAt)
	}
	if len(normalized.Tenders) != 1 || normalized.Skipped != 0 {
		t.Fatalf("unexpected batch shape: %d tenders, %d skipped", len(normalized.Tenders), normalized.Skipped)
	}
	tender := normalized.Tenders[0]
	if tender.Title != "某高速公路桥梁定期检查服务采购" {
		t.Fatalf("expected title to be trimmed, got %q", tender.Title)
	}
	if tender.ExternalSource != "zbcg" {
		t.Fatalf("expected tender to inherit run source, got %q", tender.ExternalSource)
	}
	if tender.Status != "open" {
		t.Fatalf("expected default status open, got %q", tender.Status)
	}
	if tender.PublishDate != nil || tender.Deadline != nil {
		t.Fatalf("expected blank dates to normalize to null, got %v / %v", tender.PublishDate, tender.Deadline)
	}
	for name, raw := range map[string][]byte{
		"requirement_dims": tender.RequirementDims,
		"timeline":         tender.Timeline,
		"attachments":      tender.Attachments,
		"review_result":    tender.ReviewResult,
	} {
		if string(raw) != "{}" {
			t.Fatalf("expected empty %s to normalize to {}, got %q", name, raw)
		}
	}
	if len(tender.RiskFlags) != 0 {
		t.Fatalf("expected empty risk flags, got %v", tender.RiskFlags)
	}
}

func TestNormalizeIngestRequestAllowsEmptyTenderBatch(t *testing.T) {
	req := IngestRequest{Run: CollectorRunInput{
		ExternalSource: "iccec",
		Status:         "blocked",
		Message:        "WAF 返回 418",
		StartedAt:      "2026-08-25T13:00:00Z",
	}}

	normalized, err := normalizeIngestRequest(req)
	if err != nil {
		t.Fatalf("expected blocked run without tenders to be accepted: %v", err)
	}
	if len(normalized.Tenders) != 0 || normalized.Run.Status != "blocked" {
		t.Fatalf("unexpected normalized blocked run: %+v", normalized)
	}
}

func TestNormalizeIngestRequestRejectsUnsupportedExternalSource(t *testing.T) {
	for _, source := range []string{"", "zfcg", "cebpubservice", "zbcg2"} {
		req := requestWithTenders(validTenderInput())
		req.Run.ExternalSource = source
		if _, err := normalizeIngestRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected source %q to be rejected, got %v", source, err)
		}
	}

	req := requestWithTenders(validTenderInput())
	req.Run.ExternalSource = " ICCEC "
	normalized, err := normalizeIngestRequest(req)
	if err != nil {
		t.Fatalf("expected source to be case-insensitive: %v", err)
	}
	if normalized.Run.ExternalSource != "iccec" {
		t.Fatalf("expected source to normalize to iccec, got %q", normalized.Run.ExternalSource)
	}
}

func TestNormalizeIngestRequestRejectsInvalidRunFields(t *testing.T) {
	for name, mutate := range map[string]func(*CollectorRunInput){
		"unknown status":       func(run *CollectorRunInput) { run.Status = "throttled" },
		"blank status":         func(run *CollectorRunInput) { run.Status = "" },
		"negative fetched":     func(run *CollectorRunInput) { run.FetchedCount = -1 },
		"huge fetched":         func(run *CollectorRunInput) { run.FetchedCount = maxFetchedCount + 1 },
		"blank started_at":     func(run *CollectorRunInput) { run.StartedAt = "" },
		"date only started_at": func(run *CollectorRunInput) { run.StartedAt = "2026-08-25" },
		"future started_at": func(run *CollectorRunInput) {
			run.StartedAt = time.Now().Add(48 * time.Hour).Format(time.RFC3339)
		},
		"oversized message": func(run *CollectorRunInput) {
			run.Message = strings.Repeat("超", maxRunMessageRunes+1)
		},
	} {
		req := requestWithTenders(validTenderInput())
		mutate(&req.Run)
		if _, err := normalizeIngestRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected run with %s to be rejected, got %v", name, err)
		}
	}
}

func TestNormalizeIngestRequestRejectsOversizedBatch(t *testing.T) {
	tenders := make([]TenderInput, 0, maxIngestTenders+1)
	for index := 0; index <= maxIngestTenders; index++ {
		item := validTenderInput()
		item.ExternalID = fmt.Sprintf("notice-%d", index)
		tenders = append(tenders, item)
	}

	if _, err := normalizeIngestRequest(requestWithTenders(tenders...)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected batch larger than %d to be rejected, got %v", maxIngestTenders, err)
	}
}

func TestNormalizeIngestRequestDeduplicatesExternalIDWithinBatch(t *testing.T) {
	first := validTenderInput()
	first.Title = "旧标题"
	second := validTenderInput()
	second.Title = "新标题"
	other := validTenderInput()
	other.ExternalID = "9999999999"

	normalized, err := normalizeIngestRequest(requestWithTenders(first, other, second))
	if err != nil {
		t.Fatalf("expected duplicate external_id batch to normalize: %v", err)
	}
	if len(normalized.Tenders) != 2 || normalized.Skipped != 1 {
		t.Fatalf("expected 2 tenders and 1 skipped, got %d / %d", len(normalized.Tenders), normalized.Skipped)
	}
	if normalized.Tenders[0].Title != "新标题" {
		t.Fatalf("expected later duplicate to win, got %q", normalized.Tenders[0].Title)
	}
}

func TestNormalizeIngestRequestRejectsInvalidTenderFields(t *testing.T) {
	oversizedAmount := maxBudgetAmount + 1
	negativeAmount := -1.0
	for name, mutate := range map[string]func(*TenderInput){
		"blank external_id": func(item *TenderInput) { item.ExternalID = "  " },
		"blank title":       func(item *TenderInput) { item.Title = "  " },
		"long external_id": func(item *TenderInput) {
			item.ExternalID = strings.Repeat("1", maxExternalIDRunes+1)
		},
		"long title":          func(item *TenderInput) { item.Title = strings.Repeat("标", maxTitleRunes+1) },
		"long purchaser":      func(item *TenderInput) { item.Purchaser = strings.Repeat("采", maxShortTextRunes+1) },
		"bad publish_date":    func(item *TenderInput) { item.PublishDate = "2026/08/20" },
		"bad deadline":        func(item *TenderInput) { item.Deadline = "2026-09-31" },
		"bad status":          func(item *TenderInput) { item.Status = "archived" },
		"non http source_url": func(item *TenderInput) { item.SourceURL = "javascript:alert(1)" },
		"negative budget":     func(item *TenderInput) { item.BudgetAmount = &negativeAmount },
		"huge budget":         func(item *TenderInput) { item.BudgetAmount = &oversizedAmount },
		"oversized raw_content": func(item *TenderInput) {
			item.RawContent = strings.Repeat("x", maxRawContentBytes+1)
		},
		"oversized requirement_dims": func(item *TenderInput) {
			item.RequirementDims = map[string]any{"raw": strings.Repeat("x", maxTenderJSONBytes)}
		},
		"too many risk_flags": func(item *TenderInput) {
			item.RiskFlags = make([]string, maxRiskFlagItems+1)
			for index := range item.RiskFlags {
				item.RiskFlags[index] = "风险"
			}
		},
		"long risk_flag": func(item *TenderInput) {
			item.RiskFlags = []string{strings.Repeat("风", maxRiskFlagItemRunes+1)}
		},
	} {
		item := validTenderInput()
		mutate(&item)
		if _, err := normalizeIngestRequest(requestWithTenders(item)); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected tender with %s to be rejected, got %v", name, err)
		}
	}
}

func TestNormalizeIngestRequestRejectsNullBytes(t *testing.T) {
	for name, mutate := range map[string]func(*TenderInput){
		"title":       func(item *TenderInput) { item.Title = "标题\x00注入" },
		"raw_content": func(item *TenderInput) { item.RawContent = "公告全文\x00" },
		"risk_flag":   func(item *TenderInput) { item.RiskFlags = []string{"保证金\x00"} },
		"jsonb value": func(item *TenderInput) { item.Timeline = map[string]any{"submit_deadline": "2026-09-07\x00"} },
	} {
		item := validTenderInput()
		mutate(&item)
		if _, err := normalizeIngestRequest(requestWithTenders(item)); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected NUL byte in %s to be rejected, got %v", name, err)
		}
	}
}

func TestNormalizeIngestRequestKeepsSuppliedOptionalFields(t *testing.T) {
	amount := 12_800_000.0
	item := TenderInput{
		ExternalID:      "1234567890",
		Title:           "某高速公路桥梁定期检查服务采购",
		Purchaser:       "江苏交通控股有限公司某分公司",
		Region:          "江苏",
		NoticeTypeName:  "采购(预审)公告",
		PublishDate:     "2026-08-20",
		Deadline:        "2026-09-07",
		SourceURL:       "https://zbcg.jchc.cn/notice_detailsModel?noticeId=1",
		BudgetText:      "1,280万",
		BudgetAmount:    &amount,
		RawContent:      "公告全文",
		RequirementDims: map[string]any{"confidence": "high"},
		Timeline:        map[string]any{"submit_deadline": "2026-09-07 09:30"},
		Attachments:     map[string]any{"file_price": "￥500"},
		ReviewResult:    map[string]any{"summary": map[string]any{}},
		RiskFlags:       []string{" 投标保证金比例 3% 超过 2% 上限 ", "   "},
		Status:          "CLOSED",
	}

	normalized, err := normalizeIngestRequest(requestWithTenders(item))
	if err != nil {
		t.Fatalf("expected full tender payload to normalize: %v", err)
	}
	tender := normalized.Tenders[0]
	if tender.Status != "closed" {
		t.Fatalf("expected status to lower-case, got %q", tender.Status)
	}
	if len(tender.RiskFlags) != 1 || tender.RiskFlags[0] != "投标保证金比例 3% 超过 2% 上限" {
		t.Fatalf("expected blank risk flags to be dropped and trimmed, got %v", tender.RiskFlags)
	}
	publishDate, ok := tender.PublishDate.(time.Time)
	if !ok || !publishDate.Equal(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected publish date %v", tender.PublishDate)
	}
	if string(tender.RequirementDims) != `{"confidence":"high"}` {
		t.Fatalf("unexpected requirement_dims %q", tender.RequirementDims)
	}
}

func TestClampListLimitBoundsPageSize(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want int
	}{
		{in: 0, want: defaultListLimit},
		{in: -10, want: defaultListLimit},
		{in: 1, want: 1},
		{in: maxListLimit, want: maxListLimit},
		{in: maxListLimit + 1, want: maxListLimit},
	} {
		if got := clampListLimit(tc.in); got != tc.want {
			t.Fatalf("clampListLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestClampListOffsetBoundsPagination(t *testing.T) {
	for _, tc := range []struct {
		in   int
		want int
	}{
		{in: 0, want: 0},
		{in: -5, want: 0},
		{in: 100, want: 100},
		{in: maxListOffset + 1, want: maxListOffset},
	} {
		if got := clampListOffset(tc.in); got != tc.want {
			t.Fatalf("clampListOffset(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestClampRunsLimitBoundsPageSize(t *testing.T) {
	if got := clampRunsLimit(0); got != defaultRunsLimit {
		t.Fatalf("expected default runs limit, got %d", got)
	}
	if got := clampRunsLimit(maxRunsLimit + 100); got != maxRunsLimit {
		t.Fatalf("expected runs limit to clamp, got %d", got)
	}
}

func TestNormalizeOptionalExternalSourceAllowsBlankFilter(t *testing.T) {
	source, err := normalizeOptionalExternalSource("   ")
	if err != nil || source != "" {
		t.Fatalf("expected blank source filter to be accepted, got %q / %v", source, err)
	}
	if _, err := normalizeOptionalExternalSource("zfcg"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected unknown source filter to be rejected, got %v", err)
	}
}
