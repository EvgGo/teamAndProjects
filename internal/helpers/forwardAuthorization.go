package helpers

import (
	"context"
	"google.golang.org/grpc/metadata"
)

func ForwardAuthorization(ctx context.Context) context.Context {

	inMD, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	authVals := inMD.Get("authorization")
	if len(authVals) == 0 {
		return ctx
	}

	// добавляем authorization в исходящий context
	return metadata.AppendToOutgoingContext(ctx, "authorization", authVals[0])
}
