package atlassian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAttachmentUploadRequestRejectsBothAndNeither(t *testing.T) {
	if _, err := BuildAttachmentUploadRequest("", nil, "chart.png", ""); err == nil {
		t.Fatal("expected error when neither file_path nor content_bytes")
	}
	if _, err := BuildAttachmentUploadRequest("/tmp/x", []byte("data"), "x", ""); err == nil {
		t.Fatal("expected error when both file_path and content_bytes")
	}
}

func TestBuildAttachmentUploadRequestFromBytes(t *testing.T) {
	req, err := BuildAttachmentUploadRequest("", []byte("hello"), "doc.txt", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if string(req.Data) != "hello" || req.Filename != "doc.txt" || req.ContentType == "" {
		t.Fatalf("req = %#v", req)
	}
}

func TestBuildAttachmentUploadRequestFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chart.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o600); err != nil {
		t.Fatalf("write = %v", err)
	}
	req, err := BuildAttachmentUploadRequest(path, nil, "", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if string(req.Data) != "png-bytes" || req.Filename != "chart.png" {
		t.Fatalf("req = %#v", req)
	}
	if req.ContentType != "image/png" {
		t.Fatalf("content type = %q", req.ContentType)
	}
}

func TestBuildAttachmentUploadRequestRejectsOversizeBytes(t *testing.T) {
	big := make([]byte, MaxAttachmentUploadBytes+1)
	_, err := BuildAttachmentUploadRequest("", big, "huge.bin", "application/octet-stream")
	if err == nil || !strings.Contains(err.Error(), "byte cap") {
		t.Fatalf("err = %v", err)
	}
}

func TestWriteAttachmentToFileAndDirectory(t *testing.T) {
	dir := t.TempDir()
	// Directory target → join filename.
	target := filepath.Join(dir, "sub")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir = %v", err)
	}
	written, err := WriteAttachment(target, "chart.png", []byte("data"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if written != filepath.Join(target, "chart.png") {
		t.Fatalf("written = %q", written)
	}
	if data, _ := os.ReadFile(written); string(data) != "data" {
		t.Fatalf("file contents = %q", data)
	}

	// File target → use as-is, create parent.
	file := filepath.Join(dir, "nested/out.bin")
	written, err = WriteAttachment(file, "ignored.bin", []byte("zzz"))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if written != file {
		t.Fatalf("written = %q", written)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if FirstNonEmpty("", "  ", "ok", "later") != "ok" {
		t.Fatal("expected 'ok'")
	}
	if FirstNonEmpty("", "  ") != "" {
		t.Fatal("expected empty")
	}
}
