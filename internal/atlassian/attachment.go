package atlassian

import (
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

// MaxAttachmentUploadBytes caps the size of an attachment payload assembled
// from a local file or inline bytes. Atlassian Cloud's published per-attachment
// limit (Confluence default 100 MB, Jira default 10 MB but configurable up to
// 100 MB) sits well under this; the cap is a defense against accidentally
// streaming a multi-gigabyte file through dex.
const MaxAttachmentUploadBytes = 256 * 1024 * 1024

// AttachmentUploadRequest is the normalized payload an Atlassian plugin sends
// to its multipart upload endpoint.
type AttachmentUploadRequest struct {
	Filename    string
	ContentType string
	Data        []byte
}

// BuildAttachmentUploadRequest validates and normalizes an attachment upload.
// ContentType defaults to the mime type guessed from the extension.
func BuildAttachmentUploadRequest(contentBytes []byte, filename, contentType string) (AttachmentUploadRequest, error) {
	if len(contentBytes) == 0 {
		return AttachmentUploadRequest{}, errors.New("content_bytes is required")
	}
	if int64(len(contentBytes)) > MaxAttachmentUploadBytes {
		return AttachmentUploadRequest{}, fmt.Errorf("content_bytes exceeds %d byte cap", MaxAttachmentUploadBytes)
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return AttachmentUploadRequest{}, errors.New("filename is required")
	}
	if filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") || filepath.Base(filename) != filename {
		return AttachmentUploadRequest{}, errors.New("filename must be a plain file name without path separators")
	}
	if strings.ContainsFunc(filename, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return AttachmentUploadRequest{}, errors.New("filename must not contain control characters")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	return AttachmentUploadRequest{Filename: filename, ContentType: contentType, Data: append([]byte(nil), contentBytes...)}, nil
}

// FirstNonEmpty returns the first value that has non-whitespace content,
// trimmed. Returns "" if every value is empty or whitespace.
func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
