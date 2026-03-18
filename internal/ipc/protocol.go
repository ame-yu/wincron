package ipc

import (
	"bytes"
	"encoding/json"
)

type Request struct {
	Cmd              string `json:"cmd"`
	Target           string `json:"target,omitempty"`
	Payload          string `json:"payload,omitempty"`
	ConflictStrategy string `json:"conflictStrategy,omitempty"`
}

type Response struct {
	Ok            bool   `json:"ok"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
	GlobalEnabled *bool  `json:"globalEnabled,omitempty"`
}

func marshalJSONLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func unmarshalJSONLine(data []byte, v any) error {
	return json.Unmarshal(bytes.TrimSpace(data), v)
}
