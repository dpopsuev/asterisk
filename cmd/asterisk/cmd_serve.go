package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/dpopsuev/origami/logging"
	mcpserver "asterisk/internal/mcp"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server over stdio for Cursor integration",
	Long: `Starts an MCP server over stdin/stdout. Cursor connects via .cursor/mcp.json
and calls calibration tools directly — no file-based signal protocol needed.

The server monitors for parent process death. When Cursor disconnects or
restarts its extension host, the server self-terminates to prevent zombie processes.`,
	RunE: runServe,
}

func runServe(cmd *cobra.Command, _ []string) error {
	srv := mcpserver.NewServer()
	defer srv.Shutdown()

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	mcpserver.WatchStdin(ctx, nil, cancel)

	logging.New("mcp").Info("starting asterisk MCP server over stdio (parent watchdog active)")
	return srv.MCPServer.Run(ctx, &sdkmcp.StdioTransport{})
}
