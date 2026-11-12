package grpc

import (
	"context"
	"crypto/tls"
	"math"
	"net"

	"github.com/coroot/coroot/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"
	"k8s.io/klog"
)

type Server struct {
	addr   string
	server *grpc.Server
}

func NewServer(cfg config.GRPC, tlsConfig *config.TLS) (*Server, error) {
	if cfg.Disabled {
		klog.Infoln("grpc server: disabled")
		return nil, nil
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(math.MaxInt),
		grpc.ForceServerCodecV2(rawCodec{encoding.GetCodecV2(proto.Name)}),
	}

	if tlsConfig != nil {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, err
		}
		klog.Infoln("grpc server: tls enabled")
		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		opts = append(opts, grpc.Creds(creds))

	}
	s := &Server{
		addr:   cfg.ListenAddress,
		server: grpc.NewServer(opts...),
	}
	return s, nil
}

func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl any) {
	if s == nil {
		return
	}
	s.server.RegisterService(desc, impl)
}

type rawMessage []byte

type rawCodec struct {
	encoding.CodecV2
}

func (c rawCodec) Unmarshal(data mem.BufferSlice, v any) error {
	if raw, ok := v.(*rawMessage); ok {
		*raw = data.Materialize()
		return nil
	}
	return c.CodecV2.Unmarshal(data, v)
}

type RawHandler func(ctx context.Context, request []byte) (any, error)

func (s *Server) RegisterRawMethod(service, method string, handler RawHandler) {
	if s == nil {
		return
	}
	s.server.RegisterService(&grpc.ServiceDesc{
		ServiceName: service,
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: method,
			Handler: func(_ any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				var raw rawMessage
				if err := dec(&raw); err != nil {
					return nil, err
				}
				if interceptor == nil {
					return handler(ctx, raw)
				}
				info := &grpc.UnaryServerInfo{FullMethod: "/" + service + "/" + method}
				return interceptor(ctx, []byte(raw), info, func(ctx context.Context, _ any) (any, error) {
					return handler(ctx, raw)
				})
			},
		}},
	}, nil)
}

func (s *Server) Start() error {
	if s == nil {
		return nil
	}
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	klog.Infoln("grpc server: serving on", s.addr)
	go func() {
		if err = s.server.Serve(listener); err != nil {
			klog.Exitln(err)
		}
	}()
	return nil
}

func (s *Server) Stop() {
	if s == nil {
		return
	}
	s.server.GracefulStop()
}
