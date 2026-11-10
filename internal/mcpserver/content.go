package mcpserver

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/stewartmckee/canvasbin/internal/store"
)

// pageContent returns the page source to store. gzipped nil or false leaves
// content unchanged, so callers that omit the field keep the plain-text path.
func pageContent(content string, gzipped *bool) (string, error) {
	if gzipped == nil || !*gzipped {
		return content, nil
	}
	return ungzipBase64(content)
}

func ungzipBase64(s string) (string, error) {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, s)
	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(cleaned)
	}
	if err != nil {
		return "", errors.New("gzipped content must be standard base64")
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", errors.New("gzipped content is not valid gzip")
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, int64(store.MaxContentBytes)+1))
	if err != nil {
		return "", errors.New("gzipped content is not valid gzip")
	}
	if len(out) > store.MaxContentBytes {
		return "", store.ErrTooLarge
	}
	if !utf8.Valid(out) {
		return "", errors.New("gzipped content must decompress to UTF-8 text")
	}
	return string(out), nil
}
