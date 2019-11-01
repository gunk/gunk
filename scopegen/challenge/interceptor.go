package challenge

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OAuth2Challenger interface uses the generated All method for enforcing Oauth2
type OAuth2Challenger interface {
	All(method string, claims []string) bool
}

// ClaimExtractor is a function signature for fetching the authorized claims of the request.
type ClaimExtractor func(ctx context.Context) []string

// EnforceOAuth2 creates an interceptor that ensures requests contain all scopes required by the gRPC endpoint.
func EnforceOAuth2(ext ClaimExtractor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		claims := ext(ctx)
		if chlg, ok := info.Server.(OAuth2Challenger); !ok || !chlg.All(info.FullMethod, claims) {
			return nil, status.Error(codes.PermissionDenied, "not authorized")
		}
		return handler(ctx, req)
	}
}
