// Package util provides common helper functions for file operations, HTTP, and encoding.
package util

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"

	"github.com/sirupsen/logrus"
)

// DecodeBase64Content decodes a base64-encoded string and returns the raw bytes.
func DecodeBase64Content(content string) ([]byte, error) {
	output, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("unable to decode base64: %w", err)
	}
	return output, nil
}

// DecodeGzipContent decompresses a gzip-encoded string and returns the raw bytes.
func DecodeGzipContent(content string) ([]byte, error) {
	byteContent := []byte(content)
	return DecompressGzip(byteContent)
}

// DecompressGzip decompresses gzip-compressed bytes and returns the result.
func DecompressGzip(content []byte) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("unable to decode gzip: %w", err)
	}
	defer func() {
		if err := gzr.Close(); err != nil {
			logrus.Errorf("unable to close gzip reader: %q", err)
		}
	}()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(gzr); err != nil {
		return nil, fmt.Errorf("unable to read gzip: %w", err)
	}
	return buf.Bytes(), nil
}

// DecodeContent decodes content based on the specified encoding (base64, gzip, or combined).
func DecodeContent(content string, encoding string) ([]byte, error) {
	switch encoding {
	case "":
		return []byte(content), nil
	case "b64", "base64":
		return DecodeBase64Content(content)
	case "gz", "gzip":
		return DecodeGzipContent(content)
	case "gz+base64", "gzip+base64", "gz+b64", "gzip+b64":
		gz, err := DecodeBase64Content(content)
		if err != nil {
			return nil, err
		}
		return DecodeGzipContent(string(gz))
	}
	return nil, fmt.Errorf("unsupported encoding %q", encoding)
}
