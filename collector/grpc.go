package collector

import (
	"context"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/grpc"
	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	tracesv1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/klog"
)

func (c *Collector) registerGRPCServices(server *grpc.Server) {
	server.RegisterRawMethod(tracesv1.TraceService_ServiceDesc.ServiceName, "Export", c.exportTraces)
	server.RegisterRawMethod(logsv1.LogsService_ServiceDesc.ServiceName, "Export", c.exportLogs)
}

func (c *Collector) exportTraces(ctx context.Context, request []byte) (any, error) {
	project, err := c.getProjectFromGRPCMetadata(ctx)
	if err != nil {
		klog.Errorln("failed to get project:", err)
		return nil, err
	}
	rows := getSpanRows()
	defer putSpanRows(rows)
	if err = rows.Unmarshal(request); err != nil {
		klog.Errorln(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	c.getTracesBatch(project).add(rows)
	return &tracesv1.ExportTraceServiceResponse{}, nil
}

func (c *Collector) exportLogs(ctx context.Context, request []byte) (any, error) {
	project, err := c.getProjectFromGRPCMetadata(ctx)
	if err != nil {
		klog.Errorln("failed to get project:", err)
		return nil, err
	}
	rows := getLogRows()
	defer putLogRows(rows)
	if err = rows.Unmarshal(request); err != nil {
		klog.Errorln(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	c.getLogsBatch(project).add(rows)
	return &logsv1.ExportLogsServiceResponse{}, nil
}

func (c *Collector) getProjectFromGRPCMetadata(ctx context.Context) (*db.Project, error) {
	var apiKey string
	if values := metadata.ValueFromIncomingContext(ctx, ApiKeyHeader); len(values) > 0 {
		apiKey = values[0]
	}
	return c.getProject(apiKey)
}
