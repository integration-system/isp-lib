package http

import (
	"github.com/valyala/fasthttp"
	"time"
)

type JsonRestClientOption func(client *JsonRestClient)

func WithFasttHttpEnchacer(f func(*fasthttp.Client)) JsonRestClientOption {
	return func(client *JsonRestClient) {
		f(client.c)
	}
}

func WithDefaultTimeout(timeout time.Duration) JsonRestClientOption {
	return func(client *JsonRestClient) {
		client.defaultTimeout = timeout
	}
}

type JsonRestClient struct {
	c              *fasthttp.Client
	defaultTimeout time.Duration
}

func (jrc *JsonRestClient) Invoke(method, uri string, headers map[string]string, requestBody, responsePtr interface{}) error {
	return jrc.do(method, uri, headers, requestBody, func(responseBody []byte) error {
		if responsePtr != nil {
			if err := json.Unmarshal(responseBody, responsePtr); err != nil {
				return err
			}
		}
		return nil
	})
}

func (jrc *JsonRestClient) InvokeWithoutHeaders(method, uri string, requestBody, responsePtr interface{}) error {
	return jrc.Invoke(method, uri, nil, requestBody, responsePtr)
}

func (jrc *JsonRestClient) Post(uri string, requestBody, responsePtr interface{}) error {
	return jrc.InvokeWithoutHeaders(POST, uri, requestBody, responsePtr)
}

func (jrc *JsonRestClient) Get(uri string, responsePtr interface{}) error {
	return jrc.InvokeWithoutHeaders(GET, uri, nil, responsePtr)
}

func (jrc *JsonRestClient) InvokeWithDynamicResponse(method, uri string, headers map[string]string, requestBody interface{}) (interface{}, error) {
	var res interface{}
	err := jrc.do(method, uri, headers, requestBody, func(responseBody []byte) error {
		if len(responseBody) == 0 {
			return nil
		}

		if responseBody[0] == '{' {
			res = make(map[string]interface{}, 0)
			if err := json.Unmarshal(responseBody, &res); err != nil {
				return err
			}
			return nil
		} else if responseBody[0] == '[' {
			res = make([]interface{}, 0)
			if err := json.Unmarshal(responseBody, &res); err != nil {
				return err
			}
			return nil
		} else {
			res = map[string]string{"response": string(responseBody)}
			return nil
		}
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (jrc *JsonRestClient) do(method, uri string, headers map[string]string, requestBody interface{}, respBodyHandler func([]byte) error) error {
	body, err := prepareRequestBody(requestBody)
	if err != nil {
		return err
	}

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	prepareRequest(req, method, uri, headers, body)

	if err := jrc.c.DoTimeout(req, res, jrc.defaultTimeout); err != nil {
		return err
	}

	if err := checkResponseStatusCode(res); err != nil {
		return err
	}

	return respBodyHandler(res.Body())
}

func NewJsonRestClient(opts ...JsonRestClientOption) RestClient {
	client := &JsonRestClient{
		c:              &fasthttp.Client{},
		defaultTimeout: 15 * time.Second,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func prepareRequestBody(requestBody interface{}) ([]byte, error) {
	var body []byte = nil
	if requestBody != nil {
		if bytes, err := json.Marshal(requestBody); err != nil {
			return nil, err
		} else {
			body = bytes
		}
	}
	return body, nil
}

func prepareRequest(req *fasthttp.Request, method, uri string, headers map[string]string, body []byte) {
	req.SetRequestURI(uri)
	req.Header.SetMethod(method)
	if len(headers) > 0 {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	if method != GET && body != nil {
		req.SetBody(body)
	}
}

func checkResponseStatusCode(res *fasthttp.Response) error {
	code := res.StatusCode()
	if code != fasthttp.StatusOK {
		return ErrorResponse{
			StatusCode: code,
			Status:     fasthttp.StatusMessage(code),
			Body:       string(res.Body()),
		}
	}
	return nil
}
