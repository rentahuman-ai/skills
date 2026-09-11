package knock

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestMCPToolsPersistAndDeliverMessages(t *testing.T) {
	a, b := newTestDaemon(t), newTestDaemon(t)
	ap, bp := pairDaemons(t, a, b)
	input := fmt.Sprintf("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-06-18\"}}\n{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/list\"}\n{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"knock_send\",\"arguments\":{\"peer_id\":%q,\"content\":\"sent through MCP\"}}}\n", ap.ID)
	var out bytes.Buffer
	if e := MCP(context.Background(), a.Root, bytes.NewBufferString(input), &out); e != nil {
		t.Fatal(e)
	}
	scanner := bufio.NewScanner(&out)
	responses := 0
	for scanner.Scan() {
		var reply map[string]json.RawMessage
		if e := json.Unmarshal(scanner.Bytes(), &reply); e != nil {
			t.Fatal(e)
		}
		if _, ok := reply["error"]; ok {
			t.Fatal(string(scanner.Bytes()))
		}
		if bytes.Contains(reply["result"], []byte(`"isError":true`)) {
			t.Fatal(string(scanner.Bytes()))
		}
		responses++
	}
	if responses != 3 {
		t.Fatal("notification produced a response or request was lost", responses)
	}
	eventually(t, func() bool {
		ms, _ := b.Store.Messages(bp.ID, "in", 0, 10, false)
		return len(ms) == 1 && string(ms[0].Content) == `"sent through MCP"`
	})
}
