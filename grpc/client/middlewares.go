package client

import (
	"context"
	"time"

	"github.com/integration-system/isp-lib/v3/grpc"
	"github.com/integration-system/isp-lib/v3/grpc/isp"
	"github.com/integration-system/isp-lib/v3/requestid"
	"google.golang.org/grpc/metadata"
)

func RequestId() Middleware {
	return func(next RoundTripper) RoundTripper {
		return func(ctx context.Context, message *isp.Message) (*isp.Message, error) {
			requestId := requestid.FromContext(ctx)
			if requestId == "" {
				requestId = requestid.Next()
			}

			ctx = metadata.AppendToOutgoingContext(ctx, grpc.RequestIdHeader, requestId)
			return next(ctx, message)
		}
	}
}

func DefaultTimeout(timeout time.Duration) Middleware {
	return func(next RoundTripper) RoundTripper {
		return func(ctx context.Context, message *isp.Message) (*isp.Message, error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return next(ctx, message)
		}
	}
}
