package backend

import (
	"context"
	"reflect"

	"github.com/integration-system/isp-lib/v2/isp"
	"github.com/integration-system/isp-lib/v2/streaming"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type function struct {
	dataParamType reflect.Type
	mdParamType   reflect.Type
	ctxParamNum   int
	mdParamNum    int
	dataParamNum  int
	paramsCount   int
	fun           reflect.Value
	methodName    string
}

func (f function) unmarshalAndValidateInputData(msg *isp.Message, ctx *ctx, validator Validator) (interface{}, error) {
	var dataParam interface{}
	if f.dataParamType != nil {
		val := reflect.New(f.dataParamType)
		dataParam = val.Interface()
		err := readBody(msg, dataParam)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid request body: %s", err)
		}
		if validator != nil {
			if err := validator(ctx, dataParam); err != nil {
				return nil, err
			}
		}
		return dataParam, nil
	}
	return nil, nil
}

func (f function) call(ctx context.Context, dataParam interface{}, md metadata.MD) (_ interface{}, err error) { // todo ctx context.Context
	defer func() {
		recovered := recover()
		if recovered != nil {
			err = errors.WithStack(errors.Errorf("recovered panic from handler: %v", recovered))
		}
	}()
	args := make([]reflect.Value, f.paramsCount)
	if f.mdParamNum != -1 {
		args[f.mdParamNum] = reflect.ValueOf(md).Convert(f.mdParamType)
	}
	if f.dataParamNum != -1 && dataParam != nil {
		args[f.dataParamNum] = reflect.ValueOf(dataParam).Elem()
	}
	if f.ctxParamNum != -1 {
		args[f.ctxParamNum] = reflect.ValueOf(ctx)
	}

	res := f.fun.Call(args)

	l := len(res)
	var result interface{}
	for i := 0; i < l; i++ {
		v := res[i]
		if e, ok := v.Interface().(error); ok && err == nil {
			err = e
			continue
		}
		if result == nil { // && !v.IsNil()
			result = v.Interface()
			continue
		}
	}

	return result, err
}

type streamFunction struct {
	methodName string
	consume    streaming.StreamConsumer
}
