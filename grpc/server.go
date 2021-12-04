package grpc

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/integration-system/isp-lib/v3/grpc/isp"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

type service struct {
	service atomic.Value
}

func (s *service) Request(ctx context.Context, message *isp.Message) (*isp.Message, error) {
	delegate, ok := s.service.Load().(isp.BackendServiceServer)
	if !ok {
		return nil, errors.New("handler is not initialized")
	}
	return delegate.Request(ctx, message)
}

func (s *service) RequestStream(server isp.BackendService_RequestStreamServer) error {
	delegate, ok := s.service.Load().(isp.BackendServiceServer)
	if !ok {
		return errors.New("handler is not initialized")
	}
	return delegate.RequestStream(server)
}

type Server struct {
	server  *grpc.Server
	backend *service
}

func NewServer(opts ...grpc.ServerOption) *Server {
	s := &Server{
		server: grpc.NewServer(opts...),
		backend: &service{
			service: atomic.Value{},
		},
	}
	isp.RegisterBackendServiceServer(s.server, s.backend)
	return s
}

func (s *Server) Shutdown() {
	s.server.GracefulStop()
}

func (s *Server) Upgrade(service isp.BackendServiceServer) {
	s.backend.service.Store(service)
}

func (s *Server) ListenAndServe(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.WithMessagef(err, "listen: %s", address)
	}
	return s.Serve(listener)
}

func (s *Server) Serve(listener net.Listener) error {
	err := s.server.Serve(listener)
	if err != nil {
		return errors.WithMessage(err, "serve grpc")
	}
	return nil
}
