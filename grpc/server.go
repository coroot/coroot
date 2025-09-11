package grpc

import (
	"crypto/tls"
	"math"
	"net"

	"github.com/coroot/coroot/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
