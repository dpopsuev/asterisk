package mcp

import (
	"context"
	"log"
	"os"
	"time"
)

// WatchStdin monitors for parent process death in a background goroutine.
// When the parent PID changes (Cursor disconnected or restarted its extension
// host), it calls cancelFn to trigger graceful shutdown. This prevents zombie
// MCP server processes from accumulating.
//
// IMPORTANT: This must NOT read from stdin — the MCP SDK's StdioTransport
// owns stdin exclusively. Reading from stdin here would steal bytes and corrupt
// the JSON-RPC protocol.
//
// The goroutine exits when ctx is canceled or parent death is detected.
func WatchStdin(ctx context.Context, _ any, cancelFn context.CancelFunc) {
	ppid := os.Getppid()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
				if os.Getppid() != ppid {
					log.Printf("[mcp] WARN: parent process died (was pid %d), initiating shutdown", ppid)
					cancelFn()
					return
				}
			}
		}
	}()
}
