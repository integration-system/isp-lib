package endpoint

import (
	"github.com/integration-system/isp-lib/v3/grpc/isp"
	"github.com/integration-system/isp-lib/v3/json"
	"github.com/pkg/errors"
)

type JsonResponseMapper struct {

}

func (j JsonResponseMapper) Map(result interface{}) (*isp.Message, error) {
	if result == nil {
		return &isp.Message{Body: &isp.Message_BytesBody{}}, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, errors.WithMessage(err, "marshal json")
	}
	return &isp.Message{
		Body: &isp.Message_BytesBody{
			BytesBody: data,
		},
	}, nil
}

