package qualification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound       = errors.New("qualification record not found")
	ErrInvalidRequest = errors.New("invalid qualification review request")
)

const maxReviewTextRunes = 255

// CertificateReview 是人工审核一条证书记录时提交的内容。
//
// 除 VerifyStatus 外全部用指针：nil 表示「这个字段不动」，空串表示「清空」。
// 两者必须能区分——把 OCR 抽错的证号清掉是审核时的常规操作，
// 如果用空串代表「不动」，人就永远没办法删掉一个错值。
type CertificateReview struct {
	VerifyStatus string  `json:"verify_status"`
	CertCategory *string `json:"cert_category"`
	CertName     *string `json:"cert_name"`
	CertLevel    *string `json:"cert_level"`
	CertNo       *string `json:"cert_no"`
	Issuer       *string `json:"issuer"`
	IssuedAt     *string `json:"issued_at"`
	ExpiresAt    *string `json:"expires_at"`
}

type PersonnelReview struct {
	VerifyStatus string  `json:"verify_status"`
	PersonName   *string `json:"person_name"`
	CertType     *string `json:"cert_type"`
	CertLevel    *string `json:"cert_level"`
	Major        *string `json:"major"`
	RegNo        *string `json:"reg_no"`
	ExpiresAt    *string `json:"expires_at"`
	InService    *bool   `json:"in_service"`
}

// updateBuilder 拼装只包含「本次确实要改的列」的 update 语句。
// 不用 coalesce 是因为 coalesce 无法表达「把这一列改成 NULL」。
type updateBuilder struct {
	sets []string
	args []any
}

func newUpdateBuilder(id string) *updateBuilder {
	return &updateBuilder{sets: []string{}, args: []any{id}}
}

func (b *updateBuilder) set(column string, value any) {
	b.args = append(b.args, value)
	b.sets = append(b.sets, fmt.Sprintf("%s = $%d", column, len(b.args)))
}

func normalizeVerifyStatus(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "pending_review", "confirmed", "rejected":
		return strings.TrimSpace(value), nil
	default:
		return "", ErrInvalidRequest
	}
}

func reviewText(value *string) (string, error) {
	trimmed := strings.TrimSpace(*value)
	if len([]rune(trimmed)) > maxReviewTextRunes || strings.ContainsRune(trimmed, 0) {
		return "", ErrInvalidRequest
	}
	return trimmed, nil
}

// reviewDate 解析审核表单里的日期。空串表示清空该字段。
func reviewDate(value *string) (any, error) {
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	return parsed, nil
}

// applyLevel 写入人工填写的等级原文，并由服务端算出可比 rank。
//
// 展示文本保留原文而不套用 NormalizeLevel 的归一结果：各资质体系的等级术语
// 不通用——公路养护资质分甲乙丙级，建筑业企业资质分一二三级，categoryMappings
// 里就是按各自的行业说法配的。把审核时填的「甲级」改写成「一级」，业内人看着
// 就不对，也和同类记录不一致。
//
// cert_level_rank 则绝不接受调用方传入：它决定「一级及以上」这类要求能不能判对，
// 由调用方决定等于把资格判断的正确性交了出去。甲级与一级在这里都是 RankFirst。
func applyLevel(b *updateBuilder, raw string) {
	_, rank := NormalizeLevel(raw)
	b.set("cert_level", raw)
	b.set("cert_level_rank", rank)
}

func (s *Store) ReviewCertificate(ctx context.Context, tenantID, id string, req CertificateReview) (Certificate, error) {
	status, err := normalizeVerifyStatus(req.VerifyStatus)
	if err != nil {
		return Certificate{}, err
	}
	b := newUpdateBuilder(id)
	b.set("verify_status", status)

	for _, field := range []struct {
		column string
		value  *string
	}{
		{"cert_category", req.CertCategory},
		{"cert_name", req.CertName},
		{"cert_no", req.CertNo},
		{"issuer", req.Issuer},
	} {
		if field.value == nil {
			continue
		}
		text, err := reviewText(field.value)
		if err != nil {
			return Certificate{}, err
		}
		b.set(field.column, text)
	}
	for _, field := range []struct {
		column string
		value  *string
	}{
		{"issued_at", req.IssuedAt},
		{"expires_at", req.ExpiresAt},
	} {
		if field.value == nil {
			continue
		}
		date, err := reviewDate(field.value)
		if err != nil {
			return Certificate{}, err
		}
		b.set(field.column, date)
	}
	if req.CertLevel != nil {
		level, err := reviewText(req.CertLevel)
		if err != nil {
			return Certificate{}, err
		}
		applyLevel(b, level)
	}

	query := `
update company_certificates set ` + strings.Join(b.sets, ", ") + `, updated_at = now()
where id = $1
returning id, cert_category, cert_name, cert_level, cert_level_rank, cert_no, issuer,
          issued_at, expires_at, source_ref, verify_status, extracted_by,
          extract_confidence, extract_evidence, updated_at`

	var out Certificate
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, query, b.args...)
		scanErr := row.Scan(&out.ID, &out.CertCategory, &out.CertName, &out.CertLevel, &out.CertLevelRank,
			&out.CertNo, &out.Issuer, &out.IssuedAt, &out.ExpiresAt, &out.SourceRef, &out.VerifyStatus,
			&out.ExtractedBy, &out.ExtractConfidence, &out.ExtractEvidence, &out.UpdatedAt)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return scanErr
	})
	if err != nil {
		return Certificate{}, err
	}
	out.Expired = out.ExpiresAt != nil && out.ExpiresAt.Before(time.Now())
	return out, nil
}

func (s *Store) ReviewPersonnel(ctx context.Context, tenantID, id string, req PersonnelReview) (Personnel, error) {
	status, err := normalizeVerifyStatus(req.VerifyStatus)
	if err != nil {
		return Personnel{}, err
	}
	b := newUpdateBuilder(id)
	b.set("verify_status", status)

	for _, field := range []struct {
		column string
		value  *string
	}{
		{"person_name", req.PersonName},
		{"cert_type", req.CertType},
		{"major", req.Major},
		{"reg_no", req.RegNo},
	} {
		if field.value == nil {
			continue
		}
		text, err := reviewText(field.value)
		if err != nil {
			return Personnel{}, err
		}
		b.set(field.column, text)
	}
	if req.ExpiresAt != nil {
		date, err := reviewDate(req.ExpiresAt)
		if err != nil {
			return Personnel{}, err
		}
		b.set("expires_at", date)
	}
	if req.CertLevel != nil {
		level, err := reviewText(req.CertLevel)
		if err != nil {
			return Personnel{}, err
		}
		// 人员表没有 rank 列，等级纯展示，同样保留人工填写的原文。
		b.set("cert_level", level)
	}
	if req.InService != nil {
		b.set("in_service", *req.InService)
	}

	query := `
update company_personnel set ` + strings.Join(b.sets, ", ") + `, updated_at = now()
where id = $1
returning id, person_name, cert_type, cert_level, major, reg_no, expires_at, in_service,
          source_ref, verify_status, extracted_by, extract_confidence, extract_evidence, updated_at`

	var out Personnel
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, query, b.args...)
		scanErr := row.Scan(&out.ID, &out.PersonName, &out.CertType, &out.CertLevel, &out.Major, &out.RegNo,
			&out.ExpiresAt, &out.InService, &out.SourceRef, &out.VerifyStatus, &out.ExtractedBy,
			&out.ExtractConfidence, &out.ExtractEvidence, &out.UpdatedAt)
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return scanErr
	})
	if err != nil {
		return Personnel{}, err
	}
	out.Expired = out.ExpiresAt != nil && out.ExpiresAt.Before(time.Now())
	return out, nil
}