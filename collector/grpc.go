package collector

import (
	"context"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/grpc"
	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	tracesv1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/metadata"
	"k8s.io/klog"
)

func (c *Collector) registerGRPCServices(server *grpc.Server) {
	logsv1.RegisterLogsServiceServer(server, NewGRPCLogsService(c))
	tracesv1.RegisterTraceServiceServer(server, NewGRPCTracesService(c))
}

type GRPCTracesService struct {
	collector *Collector
	tracesv1.UnimplementedTraceServiceServer
}

func NewGRPCTracesService(collector *Collector) *GRPCTracesService {
	return &GRPCTracesService{
		collector: collector,
	}
}

func (s *GRPCTracesService) Export(ctx context.Context, req *tracesv1.ExportTraceServiceRequest) (*tracesv1.ExportTraceServiceResponse, error) {
	project, err := s.collector.getProjectFromGRPCMetadata(ctx)
	if err != nil {
		klog.Errorln("failed to get project:", err)
		return nil, err
	}

	s.collector.getTracesBatch(project).Add(req)

	return &tracesv1.ExportTraceServiceResponse{}, nil
}

type GRPCLogsService struct {
	collector *Collector
	logsv1.UnimplementedLogsServiceServer
}

func NewGRPCLogsService(collector *Collector) *GRPCLogsService {
	return &GRPCLogsService{
		collector: collector,
	}
}

func (s *GRPCLogsService) Export(ctx context.Context, req *logsv1.ExportLogsServiceRequest) (*logsv1.ExportLogsServiceResponse, error) {
	project, err := s.collector.getProjectFromGRPCMetadata(ctx)
	if err != nil {
		klog.Errorln("failed to get project:", err)
		return nil, err
	}

	s.collector.getLogsBatch(project).Add(req)

	return &logsv1.ExportLogsServiceResponse{}, nil
}

func (c *Collector) getProjectFromGRPCMetadata(ctx context.Context) (*db.Project, error) {
	var apiKey string
	if values := metadata.ValueFromIncomingContext(ctx, ApiKeyHeader); len(values) > 0 {
		apiKey = values[0]
	}
	return c.getProject(apiKey)
}
