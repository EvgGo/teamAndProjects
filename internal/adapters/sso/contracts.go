package sso

import (
	"context"
	authv1 "github.com/EvgGo/proto/proto/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ViewerProfileClient interface {
	GetMe(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*authv1.User, error)
	GetProfilesByIds(ctx context.Context, in *authv1.GetProfilesByIdsRequest, opts ...grpc.CallOption) (*authv1.GetProfilesByIdsResponse, error)
}
