// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusterdropdown

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/GoogleCloudPlatform/gke-mcp/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var pendingSelections sync.Map

const (
	// htmlFilePath is the absolute path to the UI index.html file.
	// In a real production environment, this might be configurable or relative to the binary.
	htmlFilePath = "/usr/local/google/home/dharb/project/gke/agentic_space_onboarding/gke-ui-components-mcp/dist/index.html"
	resourceURI  = "ui://gke-ui-components/index.html"
	mimeType     = "text/html;profile=mcp-app"
)

type clusterDropdownArgs struct {
	Title   string   `json:"title,omitempty" jsonschema:"Title to display above the dropdown"`
	Options []string `json:"options" jsonschema:"description=List of resources to display in the dropdown"`
}

type PendingResponse struct {
	Status        string   `json:"status"`
	Options       []string `json:"options"`
	InteractionID string   `json:"interactionId"`
	Message       string   `json:"message"`
}

type submitSelectionArgs struct {
	InteractionID string `json:"interactionId" jsonschema:"The interaction ID provided by the cluster_dropdown tool."`
	Selection     string `json:"selection" jsonschema:"The user's selection."`
}

// Install registers the cluster_dropdown tool with the MCP server.
func Install(ctx context.Context, s *mcp.Server, c *config.Config) error {
	mcp.AddTool(s, &mcp.Tool{
		Name: "cluster_dropdown",
		Description: `Renders an interactive UI dropdown for the user to select an item from a list.
Use this tool when you need the user to choose one option from a set of available resources (e.g., clusters, regions, namespaces).
You MUST provide a valid array of 1 or more options. 

Timing: Call this tool immediately before you need the user's input to proceed. Do not ask the user for clarification in plain text; calling this tool serves as your question to the user.
After calling this tool, STOP and wait for the user to make a selection via the UI.
Do NOT list the options in your text response; the UI itself serves as the list and confirmation.`,
		Meta: mcp.Meta{
			"ui": map[string]interface{}{
				"resourceUri": resourceURI,
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Title to display above the dropdown",
				},
				"options": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "List of resources to display in the dropdown",
				},
			},
			"required": []string{"options"},
		},
	}, clusterDropdown)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "internal_submit_selection",
		Description: "Submit a selection for a pending interaction. This tool is called by the frontend to unblock the cluster_dropdown tool.",
	}, submitSelection)

	s.AddResource(&mcp.Resource{
		Name:     "GKE Resource Dropdown UI",
		URI:      resourceURI,
		MIMEType: mimeType,
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		htmlContent, err := os.ReadFile(htmlFilePath)

		if err != nil {
			return nil, fmt.Errorf("failed to read UI file at %s: %w", htmlFilePath, err)
		}

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				{
					URI:      resourceURI,
					MIMEType: mimeType,
					Text:     string(htmlContent),
				},
			},
		}, nil
	})

	return nil
}

func clusterDropdown(ctx context.Context, request *mcp.CallToolRequest, args *clusterDropdownArgs) (*mcp.CallToolResult, any, error) {
	// Use a constant ID for now since we can't easily communicate a generated one while blocking
	interactionID := "current"

	// Create a channel for the selection
	selectionChan := make(chan string)
	pendingSelections.Store(interactionID, selectionChan)
	defer pendingSelections.Delete(interactionID)

	// Wait for selection
	select {
	case selection := <-selectionChan:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("User selected: %s", selection),
				},
			},
		}, nil, nil
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("user selection cancelled or timed out: %w", ctx.Err())
	}
}

func submitSelection(ctx context.Context, request *mcp.CallToolRequest, args *submitSelectionArgs) (*mcp.CallToolResult, any, error) {
	// Default to "current" if not provided, or strictly require match
	id := args.InteractionID
	if id == "" {
		id = "current"
	}

	val, ok := pendingSelections.Load(id)
	if !ok {
		return nil, nil, fmt.Errorf("no pending interaction found for ID %s", id)
	}
	ch := val.(chan string)

	select {
	case ch <- args.Selection:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Selection submitted"},
			},
		}, nil, nil
	case <-ctx.Done():
		return nil, nil, fmt.Errorf("failed to submit selection (receiver not ready): %w", ctx.Err())
	}
}
