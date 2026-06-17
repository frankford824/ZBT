package aihttp

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeJSONLimitAcceptsSingleObject(t *testing.T) {
	var decoded struct {
		TaskID string `json:"task_id"`
	}

	err := DecodeJSONLimit(strings.NewReader(`{"task_id":"task-1"}   `), &decoded, 64)

	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded.TaskID != "task-1" {
		t.Fatalf("expected task_id to decode, got %q", decoded.TaskID)
	}
}

func TestDecodeJSONLimitRejectsTrailingJSON(t *testing.T) {
	var decoded map[string]any

	err := DecodeJSONLimit(strings.NewReader(`{"task_id":"task-1"} {"task_id":"task-2"}`), &decoded, 128)

	if !errors.Is(err, ErrResponseTrailingData) {
		t.Fatalf("expected trailing data error, got %v", err)
	}
}

func TestDecodeJSONLimitRejectsOversizedResponse(t *testing.T) {
	var decoded map[string]any

	err := DecodeJSONLimit(strings.NewReader(`{"value":"1234567890"}`), &decoded, 8)

	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestDecodeJSONLimitRejectsNonPositiveLimit(t *testing.T) {
	var decoded map[string]any

	err := DecodeJSONLimit(strings.NewReader(`{"ok":true}`), &decoded, 0)

	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected invalid limit to be oversized response error, got %v", err)
	}
}
