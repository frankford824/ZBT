// Package factualreview blocks generated review placeholders from final output.
// It does not rewrite user edits or certify the truth of human-entered content.
package factualreview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const Marker = "【事实待核实："

var ErrRequired = errors.New("generated facts require human review")

func CheckText(text string) error {
	if strings.Contains(strings.Join(strings.Fields(text), ""), Marker) {
		return ErrRequired
	}
	return nil
}

func CheckContent(content map[string]any) error {
	var text strings.Builder
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if value, ok := node["text"].(string); ok {
				text.WriteString(value)
			}
			visit(node["content"])
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(content)
	return CheckText(text.String())
}

func RequireClear(ctx context.Context, tx pgx.Tx, tenantID, bidID string) error {
	rows, err := tx.Query(ctx, `select plain_text, content from bid_chapters where tenant_id=$1 and bid_document_id=$2`, tenantID, bidID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var text string
		var content []byte
		if err := rows.Scan(&text, &content); err != nil {
			return err
		}
		if err := CheckText(text); err != nil {
			return err
		}
		var document map[string]any
		if len(content) > 0 {
			if err := json.Unmarshal(content, &document); err != nil {
				return err
			}
		}
		if err := CheckContent(document); err != nil {
			return err
		}
	}
	return rows.Err()
}
