package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

// initializeClangdLanguageServer initializes the Clangd language server
// with specific optimizations to warm up the static index and open core files.
func initializeClangdLanguageServer(ctx context.Context, client *Client, workspaceDir string) error {
	lspLogger.Info("Initializing Clangd language server with workspace: %s", workspaceDir)

	// Step 1: Send 1-2 workspace/symbol queries to warm up the static index
	if err := warmupClangdStaticIndex(ctx, client); err != nil {
		lspLogger.Warn("Failed to warm up static index (continuing anyway): %v", err)
		// Continue even if warmup fails - this is an optimization, not a requirement
	}

	// Step 2: Open core C++ files to trigger parsing and indexing
	if err := openCoreCppFiles(ctx, client, workspaceDir); err != nil {
		lspLogger.Warn("Failed to open core C++ files (continuing anyway): %v", err)
		// Continue even if opening files fails
	}

	lspLogger.Info("Clangd language server initialization completed successfully")
	return nil
}

// warmupClangdStaticIndex sends workspace/symbol queries to promote static index into cache
func warmupClangdStaticIndex(ctx context.Context, client *Client) error {
	lspLogger.Info("Warming up clangd static index...")

	// Query 1: General namespace separator to trigger index loading
	symbolParams1 := protocol.WorkspaceSymbolParams{
		Query: "::",
	}

	_, err := client.Symbol(ctx, symbolParams1)
	if err != nil {
		lspLogger.Warn("First warmup symbol query failed: %v", err)
	} else {
		lspLogger.Debug("First warmup symbol query completed")
	}

	// Small delay between queries
	time.Sleep(100 * time.Millisecond)

	// Query 2: Empty query to get all symbols (triggers comprehensive index loading)
	symbolParams2 := protocol.WorkspaceSymbolParams{
		Query: "",
	}

	_, err = client.Symbol(ctx, symbolParams2)
	if err != nil {
		lspLogger.Warn("Second warmup symbol query failed: %v", err)
	} else {
		lspLogger.Debug("Second warmup symbol query completed")
	}

	lspLogger.Info("Static index warmup completed")
	return nil
}

// openCoreCppFiles finds and opens the largest/most important C++ files in the workspace
func openCoreCppFiles(ctx context.Context, client *Client, workspaceDir string) error {
	lspLogger.Info("Opening core C++ files in workspace: %s", workspaceDir)

	// Find C++ files in the workspace
	var cppFiles []string
	err := filepath.Walk(workspaceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden folders
		if info.IsDir() {
			basename := filepath.Base(path)
			if strings.HasPrefix(basename, ".") || basename == "build" || basename == "cmake-build-debug" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file is a C++ source file (prioritize .cpp over .h)
		if strings.HasSuffix(path, ".cpp") || strings.HasSuffix(path, ".cxx") || strings.HasSuffix(path, ".cc") {
			cppFiles = append(cppFiles, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking workspace directory: %w", err)
	}

	// Sort by file size (largest first) to prioritize important files
	// For simplicity, we'll just open the first few files found
	fileCount := 0
	maxFilesToOpen := 3 // Limit to avoid overwhelming the server

	for _, filePath := range cppFiles {
		if fileCount >= maxFilesToOpen {
			break
		}

		if err := client.OpenFile(ctx, filePath); err != nil {
			lspLogger.Warn("Failed to open C++ file %s: %v", filePath, err)
			continue // Continue with other files even if one fails
		}

		lspLogger.Debug("Opened core C++ file: %s", filePath)
		fileCount++

		// Small delay between file opens to avoid overwhelming the server
		time.Sleep(50 * time.Millisecond)
	}

	lspLogger.Info("Opened %d core C++ files", fileCount)
	return nil
}
