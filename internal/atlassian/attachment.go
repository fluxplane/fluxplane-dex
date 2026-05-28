package atlassian

import (
	"errors"
	"fmt"
	"mime"
	"os"
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
// Exactly one of filePath / contentBytes must be set. Filename defaults to the
// file base name; ContentType defaults to the mime type guessed from the
// extension.
func BuildAttachmentUploadRequest(filePath string, contentBytes []byte, filename, contentType string) (AttachmentUploadRequest, error) {
	filePath = strings.TrimSpace(filePath)
	hasFile := filePath != ""
	hasBytes := len(contentBytes) > 0
	if hasFile == hasBytes {
		return AttachmentUploadRequest{}, errors.New("provide exactly one of file_path or content_bytes")
	}
	var data []byte
	if hasFile {
		info, err := os.Stat(filePath)
		if err != nil {
			return AttachmentUploadRequest{}, err
		}
		if info.Size() > MaxAttachmentUploadBytes {
			return AttachmentUploadRequest{}, fmt.Errorf("attachment %s exceeds %d byte cap", filePath, MaxAttachmentUploadBytes)
		}
		data, err = os.ReadFile(filePath)
		if err != nil {
			return AttachmentUploadRequest{}, err
		}
		if strings.TrimSpace(filename) == "" {
			filename = filepath.Base(filePath)
		}
	} else {
		if int64(len(contentBytes)) > MaxAttachmentUploadBytes {
			return AttachmentUploadRequest{}, fmt.Errorf("content_bytes exceeds %d byte cap", MaxAttachmentUploadBytes)
		}
		data = append([]byte(nil), contentBytes...)
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return AttachmentUploadRequest{}, errors.New("filename is required")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(filename))
	}
	return AttachmentUploadRequest{Filename: filename, ContentType: contentType, Data: data}, nil
}

// WriteAttachment writes downloaded attachment bytes to outputPath. If
// outputPath is an existing directory, filename is appended; otherwise
// outputPath is treated as the destination file. Parent directories are
// created with mode 0o755 and the file is written with mode 0o600.
func WriteAttachment(outputPath, filename string, data []byte) (string, error) {
	info, err := os.Stat(outputPath)
	if err == nil && info.IsDir() {
		outputPath = filepath.Join(outputPath, filename)
	}
	if dir := filepath.Dir(outputPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return "", err
	}
	return outputPath, nil
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
