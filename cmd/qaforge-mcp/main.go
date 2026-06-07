package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qaforge/mcp/internal/db"
	"github.com/qaforge/mcp/internal/tools"
)

func main() {
	storePath, err := defaultStorePath()
	if err != nil {
		log.Fatalf("resolve store path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		log.Fatalf("create store dir: %v", err)
	}

	conn, err := db.Open(storePath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "qaforge-mcp",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "QAForge MCP: application analysis, workflow discovery, test plan generation, Playwright execution, visual diff, database verification, bug reports, and coverage analysis.",
	})

	tools.RegisterAll(server, conn)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func defaultStorePath() (string, error) {
	if v := os.Getenv("QAFORGE_STORE"); v != "" {
		return v, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "qaforge-mcp", "store.db"), nil
}
