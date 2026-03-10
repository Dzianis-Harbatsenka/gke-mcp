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

// Package monitoring provides tools for GKE-related monitoring data.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	monitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/GoogleCloudPlatform/gke-mcp/pkg/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	htmlFilePath = "pkg/tools/ui/dist/timeserieschart/index.html"
	resourceURI  = "ui://monitoring_time_series_chart/index.html"
	mimeType     = "text/html;profile=mcp-app"
)

type handlers struct {
	c *config.Config
}

type listMonitoredResourceDescriptorsArgs struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"GCP project ID. Use the default if the user doesn't provide it."`
}

type timeSeriesChartArgs struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"GCP project ID. Use the default if the user doesn't provide it."`
	Filter    string `json:"filter" jsonschema:"Required. A monitoring filter that specifies which time series should be returned."`
	StartTime string `json:"start_time,omitempty" jsonschema:"Optional. RFC3339 formatted start time. Defaults to 1 hour before end_time."`
	EndTime   string `json:"end_time,omitempty" jsonschema:"Optional. RFC3339 formatted end time. Defaults to current time."`
	Title     string `json:"title,omitempty" jsonschema:"Optional. The title to display for the time series chart."`
}

type listTimeSeriesArgs struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"GCP project ID. Use the default if the user doesn't provide it."`
	Filter    string `json:"filter" jsonschema:"Required. A monitoring filter that specifies which time series should be returned."`
	StartTime string `json:"start_time,omitempty" jsonschema:"Optional. RFC3339 formatted start time. Defaults to 1 hour before end_time."`
	EndTime   string `json:"end_time,omitempty" jsonschema:"Optional. RFC3339 formatted end time. Defaults to current time."`
}

// Install registers monitoring tools with the MCP server.
func Install(_ context.Context, s *mcp.Server, c *config.Config) error {
	h := &handlers{
		c: c,
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_monitored_resource_descriptors",
		Description: "List monitored resource descriptors(schema) related to GKE for this project. Prefer to use this tool instead of gcloud",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
	}, h.listMRDescriptor)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "monitoring_time_series_chart",
		Description: "List time series data from Google Cloud Monitoring based on a filter and render it as a chart in the UI.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Meta: mcp.Meta{
			"ui": map[string]interface{}{
				"resourceUri": resourceURI,
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "GCP project ID. Use the default if the user doesn't provide it.",
				},
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "Required. A monitoring filter that specifies which time series should be returned.",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Optional. RFC3339 formatted start time. Defaults to 1 hour before end_time.",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "Optional. RFC3339 formatted end time. Defaults to current time.",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Optional. The title to display for the time series chart.",
				},
			},
			"required": []string{"filter", "title"},
		},
	}, h.timeSeriesChart)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_time_series",
		Description: "Internal app tool. List time series data from Google Cloud Monitoring based on a filter.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint: true,
		},
		Meta: mcp.Meta{
			"ui": map[string]interface{}{
				"visibility": []string{"app"},
			},
		},
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "GCP project ID. Use the default if the user doesn't provide it.",
				},
				"filter": map[string]interface{}{
					"type":        "string",
					"description": "Required. A monitoring filter that specifies which time series should be returned.",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Optional. RFC3339 formatted start time. Defaults to 1 hour before end_time.",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "Optional. RFC3339 formatted end time. Defaults to current time.",
				},
			},
			"required": []string{"filter"},
		},
	}, h.listTimeSeries)

	s.AddResource(&mcp.Resource{
		Name:     "Time Series Chart UI",
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

func (h *handlers) listMRDescriptor(ctx context.Context, _ *mcp.CallToolRequest, args *listMonitoredResourceDescriptorsArgs) (*mcp.CallToolResult, any, error) {
	if args.ProjectID == "" {
		args.ProjectID = h.c.DefaultProjectID()
	}
	if args.ProjectID == "" {
		return nil, nil, fmt.Errorf("project_id argument cannot be empty")
	}
	c, err := monitoring.NewMetricClient(ctx, option.WithUserAgent(h.c.UserAgent()))
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("Failed to close monitoring client: %v\n", err)
		}
	}()
	req := &monitoringpb.ListMonitoredResourceDescriptorsRequest{
		Name: fmt.Sprintf("projects/%s", args.ProjectID),
	}
	it := c.ListMonitoredResourceDescriptors(ctx, req)
	builder := new(strings.Builder)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		builder.WriteString(protojson.Format(resp))
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: builder.String()},
		},
	}, nil, nil
}

func (h *handlers) timeSeriesChart(ctx context.Context, _ *mcp.CallToolRequest, args *timeSeriesChartArgs) (*mcp.CallToolResult, any, error) {
	if args.ProjectID == "" {
		args.ProjectID = h.c.DefaultProjectID()
	}
	if args.ProjectID == "" {
		return nil, nil, fmt.Errorf("project_id argument cannot be empty")
	}
	if args.Filter == "" {
		return nil, nil, fmt.Errorf("filter argument cannot be empty")
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Rendered time series data in UI component."},
		},
	}, nil, nil
}

func (h *handlers) listTimeSeries(ctx context.Context, _ *mcp.CallToolRequest, args *listTimeSeriesArgs) (*mcp.CallToolResult, any, error) {
	if args.ProjectID == "" {
		args.ProjectID = h.c.DefaultProjectID()
	}
	if args.ProjectID == "" {
		return nil, nil, fmt.Errorf("project_id argument cannot be empty")
	}
	if args.Filter == "" {
		return nil, nil, fmt.Errorf("filter argument cannot be empty")
	}

	endTime := time.Now()
	if args.EndTime != "" {
		t, err := time.Parse(time.RFC3339, args.EndTime)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid end_time format: %w", err)
		}
		endTime = t
	}

	startTime := endTime.Add(-1 * time.Hour)
	if args.StartTime != "" {
		t, err := time.Parse(time.RFC3339, args.StartTime)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid start_time format: %w", err)
		}
		startTime = t
	}

	if startTime.After(endTime) {
		return nil, nil, fmt.Errorf("start_time cannot be after end_time")
	}

	c, err := monitoring.NewMetricClient(ctx, option.WithUserAgent(h.c.UserAgent()), option.WithQuotaProject(args.ProjectID))
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("Failed to close monitoring client: %v\n", err)
		}
	}()

	req := &monitoringpb.ListTimeSeriesRequest{
		Name:   fmt.Sprintf("projects/%s", args.ProjectID),
		Filter: args.Filter,
		Interval: &monitoringpb.TimeInterval{
			StartTime: timestamppb.New(startTime),
			EndTime:   timestamppb.New(endTime),
		},
	}


	it := c.ListTimeSeries(ctx, req)
	var series []json.RawMessage
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		b, err := protojson.Marshal(resp)
		if err != nil {
			return nil, nil, err
		}
		series = append(series, b)
	}

	resBytes, err := json.Marshal(series)
	
	if err != nil {
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resBytes)},
		},
	}, nil, nil
}
