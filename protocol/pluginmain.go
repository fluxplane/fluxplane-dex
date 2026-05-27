package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Handler func(Request) Response

func Serve(handler Handler) {
	if handler == nil {
		writeResponse(Fail("plugin_error", "handler is nil"))
		os.Exit(1)
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeResponse(Fail("read_request", err.Error()))
		os.Exit(1)
	}
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		writeResponse(Fail("bad_request", err.Error()))
		os.Exit(1)
	}
	if req.Protocol != Version {
		writeResponse(Fail("protocol_mismatch", fmt.Sprintf("expected %s, got %s", Version, req.Protocol)))
		os.Exit(1)
	}
	resp := handler(req)
	if resp.Protocol == "" {
		resp.Protocol = Version
	}
	writeResponse(resp)
	if !resp.OK {
		os.Exit(1)
	}
}

func writeResponse(resp Response) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}
