package tender

import (
	"errors"
	"net/http"
	"net/netip"
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

func TestPublicNetIPRejectsNonPublicRanges(t *testing.T) {
	for _, value := range []string{
		"0.0.0.0",
		"127.0.0.1",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.0.1",
		"169.254.169.254",
		"::1",
		"fd00::1",
	} {
		if publicNetIP(netip.MustParseAddr(value)) {
			t.Fatalf("expected %s to be rejected", value)
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
