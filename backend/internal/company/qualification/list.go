package qualification

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

type Certificate struct {
	ID                string          `json:"id"`
	CertCategory      string          `json:"cert_category"`
	CertName          string          `json:"cert_name"`
	CertLevel         string          `json:"cert_level"`
	CertLevelRank     int             `json:"cert_level_rank"`
	CertNo            string          `json:"cert_no"`
	Issuer            string          `json:"issuer"`
	IssuedAt          *time.Time      `json:"issued_at"`
	ExpiresAt         *time.Time      `json:"expires_at"`
	Expired           bool            `json:"expired"`
	SourceRef         string          `json:"source_ref"`
	VerifyStatus      string          `json:"verify_status"`
	ExtractedBy       string          `json:"extracted_by"`
	ExtractConfidence *float64        `json:"extract_confidence"`
	ExtractEvidence   json.RawMessage `json:"extract_evidence"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type Personnel struct {
	ID                string          `json:"id"`
	PersonName        string          `json:"person_name"`
	CertType          string          `json:"cert_type"`
	CertLevel         string          `json:"cert_level"`
	Major             string          `json:"major"`
	RegNo             string          `json:"reg_no"`
	ExpiresAt         *time.Time      `json:"expires_at"`
	Expired           bool            `json:"expired"`
	InService         bool            `json:"in_service"`
	SourceRef         string          `json:"source_ref"`
	VerifyStatus      string          `json:"verify_status"`
	ExtractedBy       string          `json:"extracted_by"`
	ExtractConfidence *float64        `json:"extract_confidence"`
	ExtractEvidence   json.RawMessage `json:"extract_evidence"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type ListFilter struct {
	VerifyStatus string
	Limit        int
	Offset       int
}

func (f ListFilter) normalize() ListFilter {
	if f.Limit <= 0 {
		f.Limit = defaultListLimit
	}
	if f.Limit > maxListLimit {
		f.Limit = maxListLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.VerifyStatus != "pending_review" && f.VerifyStatus != "confirmed" && f.VerifyStatus != "rejected" {
		f.VerifyStatus = ""
	}
	return f
}

type CertificateList struct {
	Items []Certificate `json:"items"`
	Total int           `json:"total"`
}

type PersonnelList struct {
	Items []Personnel `json:"items"`
	Total int         `json:"total"`
}

// ListCertificates 按待确认优先、置信度从低到高排序。
// 低置信度的排在前面是刻意的：那些恰恰是最可能抽错、最需要人眼过一遍的记录，
// 让用户从最可疑的开始看，而不是从最规整的开始看。
func (s *Store) ListCertificates(ctx context.Context, tenantID string, f ListFilter) (CertificateList, error) {
	f = f.normalize()
	out := CertificateList{Items: []Certificate{}}

	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
select count(*) from company_certificates
where ($1 = '' or verify_status = $1)`, f.VerifyStatus).Scan(&out.Total); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
select id, cert_category, cert_name, cert_level, cert_level_rank, cert_no, issuer,
       issued_at, expires_at, source_ref, verify_status, extracted_by,
       extract_confidence, extract_evidence, updated_at
from company_certificates
where ($1 = '' or verify_status = $1)
order by (verify_status = 'pending_review') desc,
         coalesce(extract_confidence, 0) asc,
         cert_level_rank desc,
         updated_at desc
limit $2 offset $3`, f.VerifyStatus, f.Limit, f.Offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		now := time.Now()
		for rows.Next() {
			var c Certificate
			if err := rows.Scan(&c.ID, &c.CertCategory, &c.CertName, &c.CertLevel, &c.CertLevelRank,
				&c.CertNo, &c.Issuer, &c.IssuedAt, &c.ExpiresAt, &c.SourceRef, &c.VerifyStatus,
				&c.ExtractedBy, &c.ExtractConfidence, &c.ExtractEvidence, &c.UpdatedAt); err != nil {
				return err
			}
			c.Expired = c.ExpiresAt != nil && c.ExpiresAt.Before(now)
			out.Items = append(out.Items, c)
		}
		return rows.Err()
	})
	return out, err
}

func (s *Store) ListPersonnel(ctx context.Context, tenantID string, f ListFilter) (PersonnelList, error) {
	f = f.normalize()
	out := PersonnelList{Items: []Personnel{}}

	err := s.withTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
select count(*) from company_personnel
where ($1 = '' or verify_status = $1)`, f.VerifyStatus).Scan(&out.Total); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
select id, person_name, cert_type, cert_level, major, reg_no, expires_at, in_service,
       source_ref, verify_status, extracted_by, extract_confidence, extract_evidence, updated_at
from company_personnel
where ($1 = '' or verify_status = $1)
order by (verify_status = 'pending_review') desc,
         coalesce(extract_confidence, 0) asc,
         person_name asc
limit $2 offset $3`, f.VerifyStatus, f.Limit, f.Offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		now := time.Now()
		for rows.Next() {
			var p Personnel
			if err := rows.Scan(&p.ID, &p.PersonName, &p.CertType, &p.CertLevel, &p.Major, &p.RegNo,
				&p.ExpiresAt, &p.InService, &p.SourceRef, &p.VerifyStatus, &p.ExtractedBy,
				&p.ExtractConfidence, &p.ExtractEvidence, &p.UpdatedAt); err != nil {
				return err
			}
			p.Expired = p.ExpiresAt != nil && p.ExpiresAt.Before(now)
			out.Items = append(out.Items, p)
		}
		return rows.Err()
	})
	return out, err
}
