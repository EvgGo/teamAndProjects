package projectsvc

import (
	"context"
	"google.golang.org/grpc/metadata"
)

func forwardViewerAuthToOutgoingContext(ctx context.Context) context.Context {
	inMD, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	outMD := metadata.MD{}

	if vals := inMD.Get("authorization"); len(vals) > 0 {
		outMD.Set("authorization", vals...)
	}

	if vals := inMD.Get("x-request-id"); len(vals) > 0 {
		outMD.Set("x-request-id", vals...)
	}

	if len(outMD) == 0 {
		return ctx
	}

	return metadata.NewOutgoingContext(ctx, outMD)
}
