package protocol

import "testing"

func TestRequestRoundTripPayload(t *testing.T) {
	req, err := NewRequest(CommandOperationsCall, "gitlab", OperationCall{Name: "gitlab.project.list"})
	if err != nil {
		t.Fatal(err)
	}
	if req.Protocol != Version {
		t.Fatalf("protocol = %q, want %q", req.Protocol, Version)
	}
	call, err := DecodePayload[OperationCall](req.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if call.Name != "gitlab.project.list" {
		t.Fatalf("operation = %q", call.Name)
	}
}

func TestFailResponse(t *testing.T) {
	resp := Fail("bad_input", "missing name")
	if resp.OK {
		t.Fatal("failure response reported OK")
	}
	if resp.Protocol != Version {
		t.Fatalf("protocol = %q", resp.Protocol)
	}
	if resp.Error == nil || resp.Error.Message != "missing name" {
		t.Fatalf("unexpected error payload: %#v", resp.Error)
	}
}

func TestBatchPayloadRoundTrip(t *testing.T) {
	req, err := NewRequest(CommandOperationsBatch, "gitlab", OperationBatch{Calls: []OperationCall{
		{ID: "a", Name: "gitlab.index.build"},
		{ID: "b", Name: "gitlab.mr.show"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := DecodePayload[OperationBatch](req.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(batch.Calls))
	}
	if batch.Calls[1].ID != "b" {
		t.Fatalf("second call id = %q", batch.Calls[1].ID)
	}
}
