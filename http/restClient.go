package http

import (
	"fmt"
	"github.com/golang/protobuf/ptypes/struct"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
)

const (
	POST = "POST"
	GET  = "GET"
)

var (
	codeMap = map[int]codes.Code{
		http.StatusOK:                  codes.OK,
		http.StatusRequestTimeout:      codes.Canceled,
		http.StatusBadRequest:          codes.InvalidArgument,
		http.StatusGatewayTimeout:      codes.DeadlineExceeded,
		http.StatusNotFound:            codes.NotFound,
		http.StatusConflict:            codes.AlreadyExists,
		http.StatusForbidden:           codes.PermissionDenied,
		http.StatusUnauthorized:        codes.Unauthenticated,
		http.StatusTooManyRequests:     codes.ResourceExhausted,
		http.StatusPreconditionFailed:  codes.FailedPrecondition,
		http.StatusNotImplemented:      codes.Unimplemented,
		http.StatusInternalServerError: codes.Internal,
		http.StatusServiceUnavailable:  codes.Unavailable,
	}
	inverseCodeMap = map[codes.Code]int{}
)

func init() {
	for httpCode, grpcCode := range codeMap {
		inverseCodeMap[grpcCode] = httpCode
	}
}

type ErrorResponse struct {
	StatusCode int
	Status     string
	Body       string
}

func (r ErrorResponse) Error() string {
	return fmt.Sprintf("statusCode:%d  status:%s  body:%s", r.StatusCode, r.Status, r.Body)
}

func (r ErrorResponse) ToGrpcError() error {
	st, _ := status.
		New(HttpStatusToCode(r.StatusCode), r.Status).
		WithDetails(&structpb.Value{Kind: &structpb.Value_StructValue{StructValue: &structpb.Struct{
			Fields: map[string]*structpb.Value{"response": {Kind: &structpb.Value_StringValue{StringValue: r.Body}}},
		}}})
	return st.Err()
}

type RestClient interface {
	Invoke(method, uri string, headers map[string]string, requestBody, responsePtr interface{}) error
	InvokeWithoutHeaders(method, uri string, requestBody, responsePtr interface{}) error
	Post(uri string, requestBody, responsePtr interface{}) error
	Get(uri string, responsePtr interface{}) error
	InvokeWithDynamicResponse(method, uri string, headers map[string]string, requestBody interface{}) (interface{}, error)
}

func HttpStatusToCode(status int) codes.Code {
	code, ok := codeMap[status]
	if !ok {
		return codes.Unknown
	}
	return code
}

func CodeToHttpStatus(code codes.Code) int {
	status, ok := inverseCodeMap[code]
	if !ok {
		return http.StatusInternalServerError
	}
	return status
}
