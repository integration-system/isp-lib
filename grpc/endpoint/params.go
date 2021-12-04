package endpoint

import (
	"context"
	"errors"

	"github.com/integration-system/isp-lib/v3/grpc"
	"github.com/integration-system/isp-lib/v3/grpc/isp"
	"google.golang.org/grpc/metadata"
)

func ContextParam() ParamMapper {
	return ParamMapper{
		Type: "context.Context",
		Builder: func(ctx context.Context, message *isp.Message) (interface{}, error) {
			return ctx, nil
		},
	}
}

func AuthDataParam() ParamMapper {
	return ParamMapper{
		Type: "grpc.AuthData",
		Builder: func(ctx context.Context, message *isp.Message) (interface{}, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return nil, errors.New("metadata is expected in context")
			}
			return grpc.AuthData(md), nil
		},
	}
}
