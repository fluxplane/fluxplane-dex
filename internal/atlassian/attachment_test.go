package atlassian

import (
	"strings"
	"testing"
)

func TestBuildAttachmentUploadRequestRejectsMissingBytes(t *testing.T) {
	if _, err := BuildAttachmentUploadRequest(nil, "chart.png", ""); err == nil {
		t.Fatal("expected error when content_bytes is missing")
	}
}

func TestBuildAttachmentUploadRequestFromBytes(t *testing.T) {
	req, err := BuildAttachmentUploadRequest([]byte("hello"), "doc.txt", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if string(req.Data) != "hello" || req.Filename != "doc.txt" || req.ContentType == "" {
		t.Fatalf("req = %#v", req)
	}
}

func TestBuildAttachmentUploadRequestRejectsOversizeBytes(t *testing.T) {
	big := make([]byte, MaxAttachmentUploadBytes+1)
	_, err := BuildAttachmentUploadRequest(big, "huge.bin", "application/octet-stream")
	if err == nil || !strings.Contains(err.Error(), "byte cap") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildAttachmentUploadRequestRejectsUnsafeFilenames(t *testing.T) {
	for _, filename := range []string{"../secret.txt", `nested\\secret.txt`, "report\n.txt", ".", ".."} {
		t.Run(filename, func(t *testing.T) {
			if _, err := BuildAttachmentUploadRequest([]byte("hello"), filename, ""); err == nil {
				t.Fatal("expected unsafe filename to be rejected")
			}
		})
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
