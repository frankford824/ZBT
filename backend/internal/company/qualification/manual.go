package qualification

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// CertificateCreate 是人工录入企业证书的最小表单。人工录入的数据直接标记为已确认，
// 并固定 extracted_by=manual，避免调用方伪造同步或 OCR 来源。
type CertificateCreate struct {
	CertCategory string `json:"cert_category"`
	CertName     string `json:"cert_name"`
	CertLevel    string `json:"cert_level"`
	CertNo       string `json:"cert_no"`
	Issuer       string `json:"issuer"`
	IssuedAt     string `json:"issued_at"`
	ExpiresAt    string `json:"expires_at"`
}

type PersonnelCreate struct {
	PersonName string `json:"person_name"`
	CertType   string `json:"cert_type"`
	CertLevel  string `json:"cert_level"`
	Major      string `json:"major"`
	RegNo      string `json:"reg_no"`
	ExpiresAt  string `json:"expires_at"`
	InService  *bool  `json:"in_service"`
}

func createText(value string, required bool) (string, error) {
	text, err := reviewText(&value)
	if err != nil || (required && strings.TrimSpace(text) == "") {
		return "", ErrInvalidRequest
	}
	return text, nil
}

func createDate(value string) (any, error) {
	return reviewDate(&value)
}

func (s *Store) CreateCertificate(ctx context.Context, tenantID string, req CertificateCreate) (Certificate, error) {
	name, err := createText(req.CertName, true)
	if err != nil {
		return Certificate{}, err
	}
	category, err := createText(req.CertCategory, false)
	if err != nil {
		return Certificate{}, err
	}
	level, err := createText(req.CertLevel, false)
	if err != nil {
		return Certificate{}, err
	}
	certNo, err := createText(req.CertNo, false)
	if err != nil {
		return Certificate{}, err
	}
	issuer, err := createText(req.Issuer, false)
	if err != nil {
		return Certificate{}, err
	}
	issuedAt, err := createDate(req.IssuedAt)
	if err != nil {
		return Certificate{}, err
	}
	expiresAt, err := createDate(req.ExpiresAt)
	if err != nil {
		return Certificate{}, err
	}
	_, rank := NormalizeLevel(level)

	var out Certificate
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
insert into company_certificates (
    tenant_id, cert_category, cert_name, cert_level, cert_level_rank, cert_no, issuer,
    issued_at, expires_at, verify_status, extracted_by, extract_evidence
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'confirmed', 'manual', '{}')
returning id, cert_category, cert_name, cert_level, cert_level_rank, cert_no, issuer,
          issued_at, expires_at, source_ref, verify_status, extracted_by,
          extract_confidence, extract_evidence, updated_at`,
			tenantID, category, name, level, rank, certNo, issuer, issuedAt, expiresAt,
		).Scan(&out.ID, &out.CertCategory, &out.CertName, &out.CertLevel, &out.CertLevelRank,
			&out.CertNo, &out.Issuer, &out.IssuedAt, &out.ExpiresAt, &out.SourceRef,
			&out.VerifyStatus, &out.ExtractedBy, &out.ExtractConfidence, &out.ExtractEvidence, &out.UpdatedAt)
	})
	if err != nil {
		return Certificate{}, err
	}
	out.Expired = out.ExpiresAt != nil && out.ExpiresAt.Before(time.Now())
	return out, nil
}

func (s *Store) CreatePersonnel(ctx context.Context, tenantID string, req PersonnelCreate) (Personnel, error) {
	name, err := createText(req.PersonName, true)
	if err != nil {
		return Personnel{}, err
	}
	certType, err := createText(req.CertType, false)
	if err != nil {
		return Personnel{}, err
	}
	level, err := createText(req.CertLevel, false)
	if err != nil {
		return Personnel{}, err
	}
	major, err := createText(req.Major, false)
	if err != nil {
		return Personnel{}, err
	}
	regNo, err := createText(req.RegNo, false)
	if err != nil {
		return Personnel{}, err
	}
	expiresAt, err := createDate(req.ExpiresAt)
	if err != nil {
		return Personnel{}, err
	}
	inService := true
	if req.InService != nil {
		inService = *req.InService
	}

	var out Personnel
	err = s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
insert into company_personnel (
    tenant_id, person_name, cert_type, cert_level, major, reg_no, expires_at,
    in_service, verify_status, extracted_by, extract_evidence
)
values ($1, $2, $3, $4, $5, $6, $7, $8, 'confirmed', 'manual', '{}')
returning id, person_name, cert_type, cert_level, major, reg_no, expires_at, in_service,
          source_ref, verify_status, extracted_by, extract_confidence, extract_evidence, updated_at`,
			tenantID, name, certType, level, major, regNo, expiresAt, inService,
		).Scan(&out.ID, &out.PersonName, &out.CertType, &out.CertLevel, &out.Major, &out.RegNo,
			&out.ExpiresAt, &out.InService, &out.SourceRef, &out.VerifyStatus, &out.ExtractedBy,
			&out.ExtractConfidence, &out.ExtractEvidence, &out.UpdatedAt)
	})
	if err != nil {
		return Personnel{}, err
	}
	out.Expired = out.ExpiresAt != nil && out.ExpiresAt.Before(time.Now())
	return out, nil
}
