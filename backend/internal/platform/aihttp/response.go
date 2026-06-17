package aihttp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const MaxResponseBytes int64 = 8 * 1024 * 1024

var (
	ErrResponseTooLarge     = errors.New("ai service response too large")
	ErrResponseTrailingData = errors.New("ai service response contains trailing data")
)

func DecodeJSON(body io.Reader, target any) error {
	return DecodeJSONLimit(body, target, MaxResponseBytes)
}

func DecodeJSONLimit(body io.Reader, target any, maxBytes int64) error {
	if maxBytes <= 0 {
		return ErrResponseTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxBytes {
		return ErrResponseTooLarge
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return ErrResponseTrailingData
}
