package cmd

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// wireRequest is the JSON wire format sent to the daemon.
type wireRequest struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

// wireResponse is the JSON wire format received from the daemon.
type wireResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

// SendRequest sends a tool request to the daemon at sockPath and returns the result.
func SendRequest(sockPath, toolName string, argsJSON json.RawMessage) (json.RawMessage, error) {
	conn, err := net.DialTimeout("unix", sockPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

	req := wireRequest{Tool: toolName, Args: argsJSON}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	var resp wireResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("%s", *resp.Error)
	}
	return resp.Result, nil
}
