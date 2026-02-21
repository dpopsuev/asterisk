package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server wraps the MCP SDK server. Domains create a server with NewServer,
// register tools via sdkmcp.AddTool(s.MCPServer, &sdkmcp.Tool{...}, handler),
// then run with s.MCPServer.Run(ctx, transport).
type Server struct {
	MCPServer *sdkmcp.Server
}

// NewServer creates an MCP server with the given implementation name and version.
// The caller registers tools using sdkmcp.AddTool(s.MCPServer, tool, handler)
// and runs the server with s.MCPServer.Run(ctx, &sdkmcp.StdioTransport{}).
func NewServer(name, version string) *Server {
	return &Server{
		MCPServer: sdkmcp.NewServer(
			&sdkmcp.Implementation{Name: name, Version: version},
			nil,
		),
	}
}
