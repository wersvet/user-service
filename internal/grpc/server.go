package igrpc

import (
	"context"
	"errors"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"user-service/internal/repositories"
	userpb "user-service/proto/user"
)

type UserGRPCServer struct {
	userpb.UnimplementedUserInternalServer
	friends    repositories.FriendRepository
	authClient *AuthClient
}

func NewUserGRPCServer(friends repositories.FriendRepository, authClient *AuthClient) *UserGRPCServer {
	return &UserGRPCServer{friends: friends, authClient: authClient}
}

func StartGRPCServer(ctx context.Context, addr string, friends repositories.FriendRepository, authClient *AuthClient) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	srv := grpc.NewServer()
	userpb.RegisterUserInternalServer(srv, NewUserGRPCServer(friends, authClient))

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	go func() {
		if err := srv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	return srv, nil
}

func (s *UserGRPCServer) AreFriends(ctx context.Context, req *userpb.AreFriendsRequest) (*userpb.AreFriendsResponse, error) {
	friends, err := s.friends.AreFriends(ctx, req.GetUserId(), req.GetFriendId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check friendship: %v", err)
	}
	return &userpb.AreFriendsResponse{AreFriends: friends}, nil
}

func (s *UserGRPCServer) GetUser(ctx context.Context, req *userpb.GetUserRequest) (*userpb.GetUserResponse, error) {
	user, err := s.authClient.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch user: %v", err)
	}
	return &userpb.GetUserResponse{Id: user.Id, Username: user.Username, CreatedAt: user.CreatedAt}, nil
}

func (s *UserGRPCServer) BulkUsers(ctx context.Context, req *userpb.BulkUsersRequest) (*userpb.BulkUsersResponse, error) {
	responses := make([]*userpb.GetUserResponse, 0, len(req.GetIds()))
	for _, id := range req.GetIds() {
		user, err := s.authClient.GetUser(ctx, id)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to fetch user %d: %v", id, err)
		}
		responses = append(responses, &userpb.GetUserResponse{Id: user.Id, Username: user.Username, CreatedAt: user.CreatedAt})
	}
	return &userpb.BulkUsersResponse{Users: responses}, nil
}
