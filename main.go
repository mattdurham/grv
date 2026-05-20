// Namespace: goast
// MCP server entrypoint (stdio transport).
package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("goast: ")
	s := server.NewMCPServer("goast", "0.1.0", server.WithToolCapabilities(false))
	RegisterTools(s)
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
