package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"easysub/internal/config"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func mcpTool(name, description string, schema map[string]any) map[string]any {
	return map[string]any{"name": name, "description": description, "inputSchema": schema}
}

func mcpTools() []map[string]any {
	empty := map[string]any{"type": "object", "properties": map[string]any{}}
	return []map[string]any{
		mcpTool("subscriptions_list", "List the authenticated user's subscriptions.", empty),
		mcpTool("dashboard_get", "Get recurring cost and renewal summary for the authenticated user.", empty),
		mcpTool("currencies_list", "List the authenticated user's configured currencies.", empty),
	}
}

func mcpResponse(c *Ctx, id json.RawMessage, result any) (any, error) {
	c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(id) == 0 || string(id) == "null" {
		c.W.WriteHeader(http.StatusAccepted)
		return Raw{}, nil
	}
	return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result}, nil
}

func mcpError(c *Ctx, id json.RawMessage, code int, message string) (any, error) {
	c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	if len(id) == 0 || string(id) == "null" {
		c.W.WriteHeader(http.StatusAccepted)
		return Raw{}, nil
	}
	return map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "error": map[string]any{"code": code, "message": message}}, nil
}

// mcp implements the JSON-RPC request/response part of Streamable HTTP MCP. It is
// deliberately stateless: API tokens are bearer credentials and no MCP session id
// is required to access a user's EasySub data.
func (s *Server) mcp(c *Ctx) (any, error) {
	if !config.C.MCPEnabled {
		return nil, Err(http.StatusNotFound, "MCP is disabled")
	}
	if c.R.Method == http.MethodGet {
		return map[string]any{"name": "EasySub MCP", "endpoint": "/mcp", "transport": "streamable-http"}, nil
	}
	if c.R.Method != http.MethodPost {
		return nil, Err(http.StatusMethodNotAllowed, "MCP requires POST")
	}

	var request mcpRequest
	if err := c.Bind(&request); err != nil {
		return nil, err
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		return mcpError(c, request.ID, -32600, "Invalid Request")
	}

	switch request.Method {
	case "initialize":
		return mcpResponse(c, request.ID, map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "easysub", "version": Version},
		})
	case "notifications/initialized":
		return mcpResponse(c, request.ID, map[string]any{"ok": true})
	case "tools/list":
		return mcpResponse(c, request.ID, map[string]any{"tools": mcpTools()})
	case "tools/call":
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return mcpError(c, request.ID, -32602, "Invalid tool parameters")
		}
		var result any
		var err error
		switch params.Name {
		case "subscriptions_list":
			result, err = s.listSubscriptions(c)
		case "dashboard_get":
			result, err = s.dashboard(c)
		case "currencies_list":
			result, err = s.listCurrencies(c)
		default:
			return mcpError(c, request.ID, -32602, "Unknown tool")
		}
		if err != nil {
			return mcpError(c, request.ID, -32603, "Tool execution failed")
		}
		data, _ := json.Marshal(result)
		return mcpResponse(c, request.ID, map[string]any{"content": []map[string]any{{"type": "text", "text": string(data)}}})
	default:
		return mcpError(c, request.ID, -32601, "Method not found")
	}
}
