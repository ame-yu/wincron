package main

import (
	"bytes"
	"encoding/json"
)

type ipcRequest struct {
	Cmd string `json:"cmd"`
}

type ipcResponse struct {
	Ok            bool  `json:"ok"`
	Message       string `json:"message,omitempty"`
	Error         string `json:"error,omitempty"`
	GlobalEnabled *bool  `json:"globalEnabled,omitempty"`
}

func marshalJSONLine(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	return b, nil
}

func unmarshalJSONLine(data []byte, v any) error {
	data = bytes.TrimSpace(data)
	return json.Unmarshal(data, v)
}
