package util

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gzipString returns the gzip-compressed form of s.
func gzipString(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write([]byte(s))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func TestDecodeBase64Content(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:    "valid base64",
			input:   base64.StdEncoding.EncodeToString([]byte("hello world")),
			want:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "invalid base64",
			input:   "not-valid-base64!!!",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			want:    []byte{},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeBase64Content(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unable to decode base64")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestDecodeGzipContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []byte
		wantErr bool
	}{
		{
			name:    "valid gzip string",
			input:   string(gzipString(t, "decompressed data")),
			want:    []byte("decompressed data"),
			wantErr: false,
		},
		{
			name:    "invalid data",
			input:   "this is not gzip data",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeGzipContent(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestDecompressGzip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []byte
		want    []byte
		wantErr bool
	}{
		{
			name:    "valid compressed bytes",
			input:   gzipString(t, "test payload"),
			want:    []byte("test payload"),
			wantErr: false,
		},
		{
			name:    "invalid bytes",
			input:   []byte("garbage data"),
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty bytes",
			input:   []byte{},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecompressGzip(tc.input)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestDecodeContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		encoding    string
		want        []byte
		wantErr     bool
		errContains string
	}{
		{
			name:     "empty encoding passthrough",
			content:  "raw content",
			encoding: "",
			want:     []byte("raw content"),
			wantErr:  false,
		},
		{
			name:     "b64 encoding",
			content:  base64.StdEncoding.EncodeToString([]byte("decoded b64")),
			encoding: "b64",
			want:     []byte("decoded b64"),
			wantErr:  false,
		},
		{
			name:     "base64 encoding",
			content:  base64.StdEncoding.EncodeToString([]byte("decoded base64")),
			encoding: "base64",
			want:     []byte("decoded base64"),
			wantErr:  false,
		},
		{
			name:     "gz encoding",
			content:  string(gzipString(t, "gz content")),
			encoding: "gz",
			want:     []byte("gz content"),
			wantErr:  false,
		},
		{
			name:     "gzip encoding",
			content:  string(gzipString(t, "gzip content")),
			encoding: "gzip",
			want:     []byte("gzip content"),
			wantErr:  false,
		},
		{
			name:     "gz+base64 combined encoding",
			content:  base64.StdEncoding.EncodeToString(gzipString(t, "combined gz+b64")),
			encoding: "gz+base64",
			want:     []byte("combined gz+b64"),
			wantErr:  false,
		},
		{
			name:     "gzip+base64 combined encoding",
			content:  base64.StdEncoding.EncodeToString(gzipString(t, "combined gzip+base64")),
			encoding: "gzip+base64",
			want:     []byte("combined gzip+base64"),
			wantErr:  false,
		},
		{
			name:     "gz+b64 combined encoding",
			content:  base64.StdEncoding.EncodeToString(gzipString(t, "combined gz+b64 alt")),
			encoding: "gz+b64",
			want:     []byte("combined gz+b64 alt"),
			wantErr:  false,
		},
		{
			name:     "gzip+b64 combined encoding",
			content:  base64.StdEncoding.EncodeToString(gzipString(t, "combined gzip+b64")),
			encoding: "gzip+b64",
			want:     []byte("combined gzip+b64"),
			wantErr:  false,
		},
		{
			name:        "unsupported encoding error",
			content:     "anything",
			encoding:    "rot13",
			want:        nil,
			wantErr:     true,
			errContains: "unsupported encoding",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeContent(tc.content, tc.encoding)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
