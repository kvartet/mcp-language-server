package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/protocol"
)

func ReadDefinition(ctx context.Context, client *lsp.Client, symbolName string) (string, error) {
	symbolResult, err := client.Symbol(ctx, protocol.WorkspaceSymbolParams{
		Query: symbolName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to fetch symbol: %v", err)
	}

	results, err := symbolResult.Results()
	if err != nil {
		return "", fmt.Errorf("failed to parse results: %v", err)
	}

	var definitions []string
	for _, symbol := range results {
		kind := ""
		container := ""

		// Skip symbols that we are not looking for. workspace/symbol may return
		// a large number of fuzzy matches.
		var containerName string

		switch v := symbol.(type) {
		case *protocol.SymbolInformation:
			// SymbolInformation results have richer data.
			kind = fmt.Sprintf("Kind: %s\n", protocol.TableKindMap[v.Kind])
			containerName = v.ContainerName
			if containerName != "" {
				container = fmt.Sprintf("Container Name: %s\n", containerName)
			}
		case *protocol.WorkspaceSymbol:
			// WorkspaceSymbol (used by clangd)
			// Only add Kind if there's a container name to distinguish from legacy output
			if v.ContainerName != "" {
				kind = fmt.Sprintf("Kind: %s\n", protocol.TableKindMap[v.Kind])
				container = fmt.Sprintf("Container Name: %s\n", v.ContainerName)
			}
			containerName = v.ContainerName
		default:
			// Unknown symbol type, use basic matching
			if symbol.GetName() != symbolName {
				continue
			}
		}

		// Trust clangd's workspace/symbol results - it already handles qualified name matching.
		// When we query "TestClass::method", clangd returns name="method" with container="TestClass"
		// When we query "method", clangd returns matching methods with their containers
		// No need for complex string parsing - just use what clangd gives us!

		// We only need minimal filtering for edge cases where clangd returns fuzzy matches
		// that are clearly not what the user intended

		// For now, accept all symbols that clangd returns for the query
		// This trusts clangd's sophisticated symbol matching algorithm

		toolsLogger.Debug("Found symbol: %s", symbol.GetName())
		loc := symbol.GetLocation()

		err := client.OpenFile(ctx, loc.URI.Path())
		if err != nil {
			toolsLogger.Error("Error opening file: %v", err)
			continue
		}

		banner := "---\n\n"
		definition, loc, err := GetFullDefinition(ctx, client, loc)
		locationInfo := fmt.Sprintf(
			"Symbol: %s\n"+
				"File: %s\n"+
				kind+
				container+
				"Range: L%d:C%d - L%d:C%d\n\n",
			symbol.GetName(),
			strings.TrimPrefix(string(loc.URI), "file://"),
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
			loc.Range.End.Line+1,
			loc.Range.End.Character+1,
		)

		if err != nil {
			toolsLogger.Error("Error getting definition: %v", err)
			continue
		}

		definition = addLineNumbers(definition, int(loc.Range.Start.Line)+1)

		definitions = append(definitions, banner+locationInfo+definition+"\n")
	}

	if len(definitions) == 0 {
		return fmt.Sprintf("%s not found", symbolName), nil
	}

	return strings.Join(definitions, ""), nil
}
