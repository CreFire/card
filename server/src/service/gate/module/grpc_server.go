package module

import (
	"context"
	"errors"
	"fmt"
	"net"

	"backend/deps/xlog"
	"backend/src/proto/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const gateControlServiceName = "pb.GateControlService"

type GateControlServiceServer interface {
	GetGamer(context.Context, *pb.GetGateGamerRequest) (*pb.GetGateGamerResponse, error)
	ListGamers(context.Context, *pb.ListGateGamersRequest) (*pb.ListGateGamersResponse, error)
	KickGamer(context.Context, *pb.KickGateGamerRequest) (*pb.KickGateGamerResponse, error)
}

func RegisterGateControlServiceServer(registrar grpc.ServiceRegistrar, srv GateControlServiceServer) {
	registrar.RegisterService(&grpc.ServiceDesc{
		ServiceName: gateControlServiceName,
		HandlerType: (*GateControlServiceServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "GetGamer",
				Handler:    getGamerHandler(srv),
			},
			{
				MethodName: "ListGamers",
				Handler:    listGamersHandler(srv),
			},
			{
				MethodName: "KickGamer",
				Handler:    kickGamerHandler(srv),
			},
		},
	}, srv)
}

type GRPCServer struct {
	addr       string
	service    *GateService
	listener   net.Listener
	grpcServer *grpc.Server
}

func NewGRPCServer(addr string, service *GateService) *GRPCServer {
	return &GRPCServer{
		addr:    addr,
		service: service,
	}
}

func (s *GRPCServer) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen grpc on %s: %w", s.addr, err)
	}

	server := grpc.NewServer()
	RegisterGateControlServiceServer(server, &gateControlGRPC{service: s.service})

	s.listener = listener
	s.grpcServer = server

	xlog.Infof("[gate] grpc listening on %s", s.addr)
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			xlog.Errorf("[gate] grpc serve failed: %v", err)
		}
	}()
	return nil
}

func (s *GRPCServer) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
}

type gateControlGRPC struct {
	service *GateService
}

func (g *gateControlGRPC) GetGamer(ctx context.Context, req *pb.GetGateGamerRequest) (*pb.GetGateGamerResponse, error) {
	gamer, err := g.service.GetGamer(ctx, req.GetConnId())
	if err != nil {
		return nil, mapGateError(err)
	}
	return &pb.GetGateGamerResponse{
		Gamer: toProtoSnapshot(gamer),
	}, nil
}

func (g *gateControlGRPC) ListGamers(ctx context.Context, _ *pb.ListGateGamersRequest) (*pb.ListGateGamersResponse, error) {
	gamers := g.service.ListGamers(ctx)
	items := make([]*pb.GateGamerSnapshot, 0, len(gamers))
	for _, gamer := range gamers {
		items = append(items, toProtoSnapshot(gamer))
	}
	return &pb.ListGateGamersResponse{
		Gamers: items,
	}, nil
}

func (g *gateControlGRPC) KickGamer(ctx context.Context, req *pb.KickGateGamerRequest) (*pb.KickGateGamerResponse, error) {
	if err := g.service.Disconnect(ctx, DisconnectInput{ConnID: req.GetConnId()}); err != nil {
		return nil, mapGateError(err)
	}
	return &pb.KickGateGamerResponse{Ok: true}, nil
}

func getGamerHandler(srv GateControlServiceServer) grpc.MethodHandler {
	return func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(pb.GetGateGamerRequest)
		if err := dec(req); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return srv.GetGamer(ctx, req)
		}
		info := &grpc.UnaryServerInfo{
			Server:     server,
			FullMethod: "/" + gateControlServiceName + "/GetGamer",
		}
		handler := func(ctx context.Context, req any) (any, error) {
			return srv.GetGamer(ctx, req.(*pb.GetGateGamerRequest))
		}
		return interceptor(ctx, req, info, handler)
	}
}

func listGamersHandler(srv GateControlServiceServer) grpc.MethodHandler {
	return func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(pb.ListGateGamersRequest)
		if err := dec(req); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return srv.ListGamers(ctx, req)
		}
		info := &grpc.UnaryServerInfo{
			Server:     server,
			FullMethod: "/" + gateControlServiceName + "/ListGamers",
		}
		handler := func(ctx context.Context, req any) (any, error) {
			return srv.ListGamers(ctx, req.(*pb.ListGateGamersRequest))
		}
		return interceptor(ctx, req, info, handler)
	}
}

func kickGamerHandler(srv GateControlServiceServer) grpc.MethodHandler {
	return func(server any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
		req := new(pb.KickGateGamerRequest)
		if err := dec(req); err != nil {
			return nil, err
		}
		if interceptor == nil {
			return srv.KickGamer(ctx, req)
		}
		info := &grpc.UnaryServerInfo{
			Server:     server,
			FullMethod: "/" + gateControlServiceName + "/KickGamer",
		}
		handler := func(ctx context.Context, req any) (any, error) {
			return srv.KickGamer(ctx, req.(*pb.KickGateGamerRequest))
		}
		return interceptor(ctx, req, info, handler)
	}
}

func toProtoSnapshot(gamer *Gamer) *pb.GateGamerSnapshot {
	if gamer == nil {
		return nil
	}
	return &pb.GateGamerSnapshot{
		ConnId:      gamer.ConnID,
		AccountId:   gamer.AccountID,
		PlayerId:    gamer.PlayerID,
		SessionId:   gamer.SessionID,
		RemoteAddr:  gamer.RemoteAddr,
		Status:      gamer.Status,
		Profile:     gamer.Profile,
		ConnectedAt: gamer.ConnectedAt.Unix(),
		LastSeenAt:  gamer.LastSeenAt.Unix(),
	}
}

func mapGateError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, ErrConnNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
