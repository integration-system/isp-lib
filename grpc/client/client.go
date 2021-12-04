package client

import (
	"context"

	"github.com/integration-system/isp-lib/v3/grpc/isp"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

const (
	resolverScheme = "isp"
	resolverUrl    = resolverScheme + ":///"
)

type Client struct {
	middlewares []Middleware
	dialOptions []grpc.DialOption

	roundTripper  RoundTripper
	hostsResolver *manual.Resolver
	grpcCli       *grpc.ClientConn
	backendCli    isp.BackendServiceClient
}

type RoundTripper func(ctx context.Context, message *isp.Message) (*isp.Message, error)

type Middleware func(next RoundTripper) RoundTripper

func New(initialHosts []string, opts ...Option) (*Client, error) {
	cli := &Client{}
	for _, opt := range opts {
		opt(cli)
	}

	hostsResolver := manual.NewBuilderWithScheme(resolverScheme)
	hostsResolver.InitialState(resolver.State{
		Addresses: toAddresses(initialHosts),
	})
	dialOptions := cli.dialOptions
	dialOptions = append(
		dialOptions,
		grpc.WithResolvers(hostsResolver),
	)

	grpcCli, err := grpc.Dial(resolverUrl, dialOptions...)
	if err != nil {
		return nil, errors.WithMessage(err, "dial grpc")
	}
	backendCli := isp.NewBackendServiceClient(grpcCli)

	cli.hostsResolver = hostsResolver
	cli.backendCli = backendCli
	cli.grpcCli = grpcCli

	roundTripper := cli.do
	for i := len(cli.middlewares) - 1; i >= 0; i-- {
		roundTripper = cli.middlewares[i](roundTripper)
	}
	cli.roundTripper = roundTripper

	return cli, nil
}

func (cli *Client) Invoke(endpoint string) *RequestBuilder {
	return NewRequestBuilder(cli.roundTripper, endpoint)
}

func (cli *Client) Upgrade(hosts []string) {
	cli.hostsResolver.UpdateState(resolver.State{
		Addresses: toAddresses(hosts),
	})
}

func (cli *Client) Close() error {
	return cli.grpcCli.Close()
}

func (cli *Client) do(ctx context.Context, message *isp.Message) (*isp.Message, error) {
	return cli.backendCli.Request(ctx, message)
}

func toAddresses(hosts []string) []resolver.Address {
	addresses := make([]resolver.Address, len(hosts))
	for i, host := range hosts {
		addresses[i] = resolver.Address{
			Addr: host,
		}
	}
	return addresses
}
