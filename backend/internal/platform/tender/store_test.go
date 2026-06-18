package tender

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestValidHTTPURLAcceptsPublicHTTPHosts(t *testing.T) {
	for _, value := range []string{
		"https://example.com/tenders/1",
		"http://www.cebpubservice.com/source",
		"https://8.8.8.8/status",
	} {
		if !validHTTPURL(value) {
			t.Fatalf("expected %q to be accepted", value)
		}
	}
}

func TestValidHTTPURLRejectsLocalAndPrivateHosts(t *testing.T) {
	for _, value := range []string{
		"http://localhost:9000",
		"http://127.0.0.1:9000",
		"http://10.0.0.8/admin",
		"http://172.16.0.8/admin",
		"http://192.168.1.20/admin",
		"http://[::1]/admin",
		"http://[fd00::1]/admin",
		"http://minio:9000",
		"http://metadata.local/path",
		"file:///etc/passwd",
	} {
		if validHTTPURL(value) {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestValidHTTPURLRejectsSpecialUseIPHosts(t *testing.T) {
	for _, value := range []string{
		"http://100.64.0.1/admin",
		"http://192.0.2.1/admin",
		"http://198.18.0.1/admin",
		"http://198.51.100.1/admin",
		"http://203.0.113.1/admin",
		"http://240.0.0.1/admin",
		"http://[2001:db8::1]/admin",
		"http://[2002::1]/admin",
	} {
		if validHTTPURL(value) {
			t.Fatalf("expected special-use host %q to be rejected", value)
		}
	}
}

func TestValidHTTPURLRejectsUserinfo(t *testing.T) {
	for _, value := range []string{
		"https://user:pass@example.com/source",
		"https://127.0.0.1@example.com/source",
		"https://example.com@127.0.0.1/path",
	} {
		if validHTTPURL(value) {
			t.Fatalf("expected URL with userinfo %q to be rejected", value)
		}
	}
}

func TestPublicNetIPRejectsNonPublicRanges(t *testing.T) {
	for _, value := range []string{
		"0.0.0.0",
		"0.0.0.1",
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.0.1",
		"169.254.169.254",
		"100.64.0.1",
		"192.0.2.1",
		"198.18.0.1",
		"198.51.100.1",
		"203.0.113.1",
		"240.0.0.1",
		"::1",
		"fd00::1",
		"2001:db8::1",
		"2002::1",
	} {
		if publicNetIP(netip.MustParseAddr(value)) {
			t.Fatalf("expected %s to be rejected", value)
		}
	}
}

func TestNormalizeTenderWriteRequestValidatesOptionalSourceURL(t *testing.T) {
	base := CreateTenderRequest{
		Title:  "测试标讯",
		Status: "open",
	}

	normalized, err := normalizeTenderWriteRequest(base)
	if err != nil {
		t.Fatalf("expected blank source URL to normalize: %v", err)
	}
	if normalized.SourceURL != "" {
		t.Fatalf("expected blank source URL to stay empty, got %q", normalized.SourceURL)
	}

	withPublicURL := base
	withPublicURL.SourceURL = " https://example.com/tender "
	normalized, err = normalizeTenderWriteRequest(withPublicURL)
	if err != nil {
		t.Fatalf("expected public source URL to normalize: %v", err)
	}
	if normalized.SourceURL != "https://example.com/tender" {
		t.Fatalf("expected source URL to be trimmed, got %q", normalized.SourceURL)
	}

	for _, value := range []string{
		"javascript:alert(1)",
		"http://127.0.0.1/admin",
		"http://100.64.0.1/admin",
		"https://user@example.com/tender",
		"https://example.com/" + strings.Repeat("a", maxTenderURLRunes),
	} {
		req := base
		req.SourceURL = value
		if _, err := normalizeTenderWriteRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected source URL %q to be rejected, got %v", value, err)
		}
	}
}

func TestNormalizeTenderWriteRequestTrimsBusinessFieldsAndLists(t *testing.T) {
	amount := 1200.0
	normalized, err := normalizeTenderWriteRequest(CreateTenderRequest{
		Title:        " 智慧交通项目 ",
		Purchaser:    " 采购单位 ",
		Region:       " 浙江 ",
		BudgetAmount: &amount,
		BudgetText:   " 1200 万 ",
		Summary:      " 项目摘要 ",
		Requirements: []string{" 资质要求 ", " ", "业绩要求"},
		RiskFlags:    []string{" 废标风险 ", "\t"},
		Metadata:     map[string]any{" source ": "manual"},
	})
	if err != nil {
		t.Fatalf("expected bounded tender request to normalize: %v", err)
	}
	if normalized.Title != "智慧交通项目" || normalized.Purchaser != "采购单位" || normalized.Region != "浙江" {
		t.Fatalf("expected tender business fields to be trimmed, got %+v", normalized)
	}
	if normalized.BudgetText != "1200 万" || normalized.Summary != "项目摘要" {
		t.Fatalf("expected budget text and summary to be trimmed, got %+v", normalized)
	}
	if len(normalized.Requirements) != 2 || normalized.Requirements[0] != "资质要求" || normalized.Requirements[1] != "业绩要求" {
		t.Fatalf("expected requirements to be trimmed and blank values removed, got %v", normalized.Requirements)
	}
	if len(normalized.RiskFlags) != 1 || normalized.RiskFlags[0] != "废标风险" {
		t.Fatalf("expected risk flags to be trimmed and blank values removed, got %v", normalized.RiskFlags)
	}
	var metadata map[string]any
	if err := json.Unmarshal(normalized.Metadata, &metadata); err != nil {
		t.Fatalf("decode normalized metadata: %v", err)
	}
	if metadata["source"] != "manual" {
		t.Fatalf("expected metadata key to be trimmed, got %#v", metadata)
	}
}

func TestCreateTenderRejectsOversizedBusinessFieldsBeforeDB(t *testing.T) {
	store := NewStore(nil)
	_, err := store.Create(context.Background(), "tenant-id", "user-id", CreateTenderRequest{
		Title:   "测试标讯",
		Summary: strings.Repeat("摘", maxTenderSummaryRunes+1),
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected oversized tender summary to be rejected before DB, got %v", err)
	}
}

func TestNormalizeTenderWriteRequestRejectsOversizedBusinessFields(t *testing.T) {
	base := CreateTenderRequest{Title: "测试标讯"}
	tooManyListItems := make([]string, maxTenderListItems+1)
	for i := range tooManyListItems {
		tooManyListItems[i] = "要求"
	}
	for _, req := range []CreateTenderRequest{
		{Title: strings.Repeat("标", maxTenderTitleRunes+1)},
		{Title: base.Title, Purchaser: strings.Repeat("采", maxTenderShortTextRunes+1)},
		{Title: base.Title, Region: strings.Repeat("区", maxTenderShortTextRunes+1)},
		{Title: base.Title, BudgetText: strings.Repeat("预", maxTenderShortTextRunes+1)},
		{Title: base.Title, Summary: strings.Repeat("摘", maxTenderSummaryRunes+1)},
		{Title: base.Title, Requirements: tooManyListItems},
		{Title: base.Title, Requirements: []string{strings.Repeat("要", maxTenderListItemRunes+1)}},
		{Title: base.Title, RiskFlags: tooManyListItems},
		{Title: base.Title, RiskFlags: []string{strings.Repeat("险", maxTenderListItemRunes+1)}},
	} {
		if _, err := normalizeTenderWriteRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected oversized tender request to be rejected, got %v for %+v", err, req)
		}
	}
}

func TestNormalizeTenderWriteRequestRejectsInvalidMetadataAndBudget(t *testing.T) {
	tooManyMetadata := make(map[string]any, maxTenderMetadataEntries+1)
	for i := 0; i <= maxTenderMetadataEntries; i++ {
		tooManyMetadata[fmt.Sprintf("key-%d", i)] = i
	}
	negative := -1.0
	tooHigh := maxTenderBudgetAmount + 1
	nan := math.NaN()
	inf := math.Inf(1)
	for _, req := range []CreateTenderRequest{
		{Title: "测试标讯", Metadata: "bad metadata"},
		{Title: "测试标讯", Metadata: map[string]any{" ": "blank key"}},
		{Title: "测试标讯", Metadata: map[string]any{strings.Repeat("键", maxTenderMetadataKeyRunes+1): "long key"}},
		{Title: "测试标讯", Metadata: map[string]any{"payload": strings.Repeat("元", maxTenderMetadataJSONBytes)}},
		{Title: "测试标讯", Metadata: map[string]any{"bad": func() {}}},
		{Title: "测试标讯", Metadata: tooManyMetadata},
		{Title: "测试标讯", BudgetAmount: &negative},
		{Title: "测试标讯", BudgetAmount: &tooHigh},
		{Title: "测试标讯", BudgetAmount: &nan},
		{Title: "测试标讯", BudgetAmount: &inf},
	} {
		if _, err := normalizeTenderWriteRequest(req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected invalid tender metadata or budget to be rejected, got %v for %+v", err, req)
		}
	}
}

func TestNormalizeSourceWriteErrorMapsConstraintFailures(t *testing.T) {
	for _, code := range []string{"23503", "23505"} {
		err := normalizeSourceWriteError(&pgconn.PgError{Code: code})
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected %s to map to ErrInvalidRequest, got %v", code, err)
		}
	}
}

func TestNormalizeTenderStatusRejectsUnsupportedValues(t *testing.T) {
	status, err := normalizeTenderStatus(" ")
	if err != nil {
		t.Fatalf("expected blank tender status to be accepted: %v", err)
	}
	if status != "" {
		t.Fatalf("expected blank tender status to stay empty for caller defaulting, got %q", status)
	}

	status, err = normalizeTenderStatus(" AWARDED ")
	if err != nil {
		t.Fatalf("expected known tender status to normalize: %v", err)
	}
	if status != "awarded" {
		t.Fatalf("expected known tender status to normalize to awarded, got %q", status)
	}

	if _, err := normalizeTenderStatus("unknown"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected unsupported tender status to be rejected, got %v", err)
	}
}

func TestMergeTenderUpdateRequestPreservesOmittedFields(t *testing.T) {
	budget := 1200.0
	deadline := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	status := " AWARDED "
	summary := "更新摘要"
	merged := mergeTenderUpdateRequest(Tender{
		Title:        "原标讯",
		Purchaser:    "原采购人",
		Region:       "浙江",
		BudgetAmount: &budget,
		BudgetText:   "1200 万",
		Deadline:     &deadline,
		Status:       "open",
		MatchScore:   67,
		Summary:      "原摘要",
		Requirements: []string{"资质"},
		RiskFlags:    []string{"风险"},
		SourceURL:    "https://example.com/tender",
		Metadata:     map[string]any{"source": "manual"},
	}, UpdateTenderRequest{
		Status:  &status,
		Summary: &summary,
	})
	normalized, err := normalizeTenderWriteRequest(merged)
	if err != nil {
		t.Fatalf("expected merged tender update to normalize: %v", err)
	}
	if normalized.Title != "原标讯" || normalized.Purchaser != "原采购人" || normalized.BudgetAmount == nil || *normalized.BudgetAmount != 1200 {
		t.Fatalf("expected omitted fields to be preserved, got %+v", normalized)
	}
	if normalized.Status != "awarded" || normalized.Summary != "更新摘要" {
		t.Fatalf("expected provided fields to be applied, got %+v", normalized)
	}
}

func TestMergeTenderUpdateRequestRejectsBlankProvidedTitle(t *testing.T) {
	title := " "
	merged := mergeTenderUpdateRequest(Tender{
		Title:  "原标讯",
		Status: "open",
	}, UpdateTenderRequest{Title: &title})
	if _, err := normalizeTenderWriteRequest(merged); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected blank provided title to be rejected, got %v", err)
	}
}

func TestNormalizeSourceStatusDefaultsOnlyBlankStatus(t *testing.T) {
	status, err := normalizeSourceStatus(" ")
	if err != nil {
		t.Fatalf("expected blank source status to default: %v", err)
	}
	if status != "active" {
		t.Fatalf("expected blank source status to default to active, got %q", status)
	}

	status, err = normalizeSourceStatus(" FAILED ")
	if err != nil {
		t.Fatalf("expected known source status to normalize: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected known source status to normalize to failed, got %q", status)
	}

	if _, err := normalizeSourceStatus("unknown"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected unsupported source status to be rejected, got %v", err)
	}
}

func TestMergeSourceUpdateRequestPreservesOmittedFields(t *testing.T) {
	name := "新来源"
	status := " INACTIVE "
	merged := mergeSourceUpdateRequest(Source{
		Name:       "原来源",
		SourceType: "招标平台",
		URL:        "https://example.com/source",
		Status:     "active",
		Config:     map[string]any{"region": "浙江"},
	}, UpdateSourceRequest{
		Name:   &name,
		Status: &status,
	})
	normalized, err := normalizeSourceWriteRequest(merged)
	if err != nil {
		t.Fatalf("expected merged source update to normalize: %v", err)
	}
	if normalized.Name != "新来源" || normalized.Status != "inactive" {
		t.Fatalf("expected provided source fields to be applied, got %+v", normalized)
	}
	if normalized.SourceType != "招标平台" || normalized.URL != "https://example.com/source" {
		t.Fatalf("expected omitted source fields to be preserved, got %+v", normalized)
	}
}

func TestNormalizeSourceWriteRequestTrimsIdentityAndDefaultsType(t *testing.T) {
	normalized, err := normalizeSourceWriteRequest(CreateSourceRequest{
		Name:   " 全国招标来源 ",
		URL:    " https://example.com/source ",
		Status: " active ",
	})
	if err != nil {
		t.Fatalf("expected bounded source request to normalize: %v", err)
	}
	if normalized.Name != "全国招标来源" || normalized.URL != "https://example.com/source" {
		t.Fatalf("expected source name and URL to be trimmed, got %+v", normalized)
	}
	if normalized.SourceType != "其他" {
		t.Fatalf("expected blank source type to default, got %q", normalized.SourceType)
	}

	normalized, err = normalizeSourceWriteRequest(CreateSourceRequest{
		Name:       "全国招标来源",
		SourceType: " 招标平台 ",
		URL:        "https://example.com/source",
		Status:     "active",
	})
	if err != nil {
		t.Fatalf("expected bounded source type to normalize: %v", err)
	}
	if normalized.SourceType != "招标平台" {
		t.Fatalf("expected source type to be trimmed, got %q", normalized.SourceType)
	}
}

func TestCreateSourceRejectsOversizedIdentityBeforeDB(t *testing.T) {
	store := NewStore(nil)
	valid := CreateSourceRequest{
		Name:       "全国招标来源",
		SourceType: "招标平台",
		URL:        "https://example.com/source",
		Status:     "active",
	}
	for _, req := range []CreateSourceRequest{
		{Name: strings.Repeat("源", maxTenderSourceNameRunes+1), SourceType: valid.SourceType, URL: valid.URL, Status: valid.Status},
		{Name: valid.Name, SourceType: strings.Repeat("类", maxTenderSourceTypeRunes+1), URL: valid.URL, Status: valid.Status},
		{Name: valid.Name, SourceType: valid.SourceType, URL: "https://example.com/" + strings.Repeat("a", maxTenderURLRunes), Status: valid.Status},
	} {
		if _, err := store.CreateSource(context.Background(), "tenant-id", req); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected oversized source request to be rejected before DB, got %v for %+v", err, req)
		}
	}
}

func TestMergeSourceUpdateRequestRejectsInvalidProvidedURL(t *testing.T) {
	url := "http://127.0.0.1/admin"
	merged := mergeSourceUpdateRequest(Source{
		Name:       "原来源",
		SourceType: "招标平台",
		URL:        "https://example.com/source",
		Status:     "active",
	}, UpdateSourceRequest{URL: &url})
	if _, err := normalizeSourceWriteRequest(merged); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid provided url to be rejected, got %v", err)
	}
}

func TestCreateSourceRejectsOversizedConfigBeforeDB(t *testing.T) {
	store := NewStore(nil)
	req := CreateSourceRequest{
		Name:       "全国招标来源",
		SourceType: "招标平台",
		URL:        "https://example.com/source",
		Status:     "active",
		Config:     map[string]any{"payload": strings.Repeat("配", maxSourceConfigJSONBytes)},
	}
	if _, err := store.CreateSource(context.Background(), "tenant-id", req); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected oversized source config to be rejected before DB, got %v", err)
	}
}

func TestNormalizeSourceConfigTrimsKeysAndBoundsJSON(t *testing.T) {
	raw, err := normalizeSourceConfig(map[string]any{
		" region ": "浙江",
		"enabled":  true,
	})
	if err != nil {
		t.Fatalf("expected bounded source config to normalize: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode normalized config: %v", err)
	}
	if decoded["region"] != "浙江" || decoded["enabled"] != true {
		t.Fatalf("expected normalized config keys and values, got %#v", decoded)
	}

	raw, err = normalizeSourceConfig(nil)
	if err != nil {
		t.Fatalf("expected nil source config to normalize: %v", err)
	}
	if string(raw) != "{}" {
		t.Fatalf("expected nil source config to normalize to {}, got %s", raw)
	}
}

func TestNormalizeSourceConfigRejectsInvalidShape(t *testing.T) {
	tooMany := make(map[string]any, maxSourceConfigEntries+1)
	for i := 0; i <= maxSourceConfigEntries; i++ {
		tooMany[fmt.Sprintf("key-%d", i)] = i
	}
	for _, config := range []map[string]any{
		{" ": "blank key"},
		{strings.Repeat("键", maxSourceConfigKeyRunes+1): "long key"},
		{"payload": strings.Repeat("配", maxSourceConfigJSONBytes)},
		{"bad": func() {}},
		tooMany,
	} {
		if _, err := normalizeSourceConfig(config); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected invalid source config to be rejected, got %v", err)
		}
	}
}

func TestSourceVerifyOutcomeUsesUserFacingMessages(t *testing.T) {
	for _, tc := range []struct {
		name        string
		resp        *http.Response
		err         error
		wantStatus  string
		wantMessage string
	}{
		{
			name:        "transport error",
			err:         errors.New("dial tcp 10.0.0.1:443: connect: connection refused"),
			wantStatus:  "failed",
			wantMessage: sourceVerifyUnavailableMessage,
		},
		{
			name:        "no response",
			wantStatus:  "failed",
			wantMessage: sourceVerifyUnavailableMessage,
		},
		{
			name:        "bad status",
			resp:        &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error"},
			wantStatus:  "failed",
			wantMessage: sourceVerifyFailedMessage,
		},
		{
			name:        "success",
			resp:        &http.Response{StatusCode: http.StatusOK, Status: "200 OK"},
			wantStatus:  "ok",
			wantMessage: sourceVerifySuccessMessage,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, message := sourceVerifyOutcome(tc.resp, tc.err)
			if status != tc.wantStatus {
				t.Fatalf("expected status %q, got %q", tc.wantStatus, status)
			}
			if message != tc.wantMessage {
				t.Fatalf("expected message %q, got %q", tc.wantMessage, message)
			}
		})
	}
}
