package backend

import (
	isp "github.com/integration-system/isp-lib/proto/stubs"
	"github.com/integration-system/isp-lib/utils"
	"google.golang.org/grpc/metadata"
	"reflect"
)

type function struct {
	dataParamType reflect.Type
	mdParamType   reflect.Type
	mdParamNum    int
	dataParamNum  int
	fun           reflect.Value
	methodName    string
}

func (f function) unmarshalAndValidateInputData(msg *isp.Message) (interface{}, error) {
	var dataParam interface{}
	if f.dataParamType != nil {
		val := reflect.New(f.dataParamType)
		dataParam = val.Interface()
		err := readBody(msg, dataParam)
		if err != nil {
			return nil, err
		}
		err = utils.Validate(dataParam)
		if err != nil {
			return nil, err
		}
		return dataParam, nil
	}
	return nil, nil
}

func (f function) call(dataParam interface{}, md metadata.MD) (interface{}, error) {
	var argCount int
	if f.mdParamNum > f.dataParamNum {
		argCount = f.mdParamNum + 1
	} else {
		argCount = f.dataParamNum + 1
	}
	args := make([]reflect.Value, argCount)
	if f.mdParamNum != -1 {
		args[f.mdParamNum] = reflect.ValueOf(md).Convert(f.mdParamType)
	}
	if f.dataParamNum != -1 && dataParam != nil {
		args[f.dataParamNum] = reflect.ValueOf(dataParam).Elem()
	}

	res := f.fun.Call(args)

	l := len(res)
	var result interface{}
	var err error
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
