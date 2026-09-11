package knock

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func MCP(ctx context.Context, root string, in io.Reader, out io.Writer) error {
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), MaxMessageBytes+16384)
	enc := json.NewEncoder(out)
	props := map[string]any{"peer_id": map[string]string{"type": "string"}, "url": map[string]string{"type": "string"}, "code": map[string]string{"type": "string", "description": "One-time pairing code; omit from logs and user-visible arguments where the host cannot protect secrets."}, "content": map[string]any{}, "reply_to": map[string]string{"type": "string"}, "after": map[string]string{"type": "integer"}, "limit": map[string]string{"type": "integer"}, "timeout_seconds": map[string]string{"type": "integer"}, "direction": map[string]string{"type": "string"}, "next_wake_at": map[string]string{"type": "string"}}
	methods := map[string]string{"invite": "Create an invitation. The returned code is sensitive; present separately to the owner.", "send": "Send text or JSON to a paired peer.", "inbox": "Read incoming or outgoing messages with a sequence cursor.", "wait": "Wait up to 60 seconds for new messages.", "status": "Read connector and runtime status.", "doctor": "Inspect networking and runtime configuration.", "stop": "Persistently stop the connector and its workers.", "revoke": "Revoke a peer's access.", "schedule": "Schedule the agent's next wakeup.", "refresh": "Update a paired peer endpoint without changing its identity."}
	for scan.Scan() {
		var req rpcRequest
		if e := json.Unmarshal(scan.Bytes(), &req); e != nil {
			if e = enc.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32700, "message": "Parse error"}}); e != nil {
				return e
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		response := map[string]any{"jsonrpc": "2.0", "id": req.ID}
		var result any
		var callError error
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "knock", "version": Version}}
		case "ping":
			result = map[string]any{}
		case "tools/list":
			tools := []any{}
			for method, desc := range methods {
				required := []string{}
				switch method {
				case "send":
					required = []string{"peer_id", "content"}
				case "inbox", "wait", "revoke":
					required = []string{"peer_id"}
				case "schedule":
					required = []string{"peer_id", "next_wake_at"}
				case "refresh":
					required = []string{"url"}
				}
				tools = append(tools, map[string]any{"name": "knock_" + method, "description": desc, "inputSchema": map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}})
			}
			result = map[string]any{"tools": tools}
		case "tools/call":
			var p struct {
				Name      string `json:"name"`
				Arguments Call   `json:"arguments"`
			}
			if e := json.Unmarshal(req.Params, &p); e != nil {
				callError = e
				break
			}
			method := ""
			for m := range methods {
				if p.Name == "knock_"+m {
					method = m
				}
			}
			if method == "" {
				callError = fmt.Errorf("unknown tool")
				break
			}
			data, e := Control(ctx, root, method, p.Arguments)
			content := string(data)
			if e != nil {
				content = e.Error()
			}
			result = map[string]any{"content": []any{map[string]string{"type": "text", "text": content}}, "isError": e != nil}
		default:
			callError = fmt.Errorf("method not found")
		}
		if callError != nil {
			response["error"] = map[string]any{"code": -32601, "message": callError.Error()}
		} else {
			response["result"] = result
		}
		if e := enc.Encode(response); e != nil {
			return e
		}
	}
	return scan.Err()
}
