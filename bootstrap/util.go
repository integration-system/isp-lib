package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cenkalti/backoff"
	etp "github.com/integration-system/isp-etp-go/client"
	"github.com/integration-system/isp-lib/backend"
	"github.com/integration-system/isp-lib/structure"
	"github.com/integration-system/isp-lib/utils"
	"github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"math/rand"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func assertSingleParamFunc(rt reflect.Type, expectingType string) {
	if rt.Kind() != reflect.Func ||
		rt.NumIn() != 1 ||
		rt.In(0).String() != expectingType {
		panic(fmt.Errorf("expecting function with one parameter of '%s' type, received '%s'", expectingType, rt.In(0).String()))
	}
}

func assertTwoParamFunc(rt reflect.Type, expectingType string) {
	if rt.Kind() != reflect.Func ||
		rt.NumIn() != 2 ||
		rt.In(0).String() != expectingType ||
		rt.In(1).String() != expectingType {
		panic(fmt.Errorf("expecting function with two '%s' parameters", expectingType))
	}
}

func callFunc(f *reflect.Value, args ...interface{}) {
	values := make([]reflect.Value, len(args))
	for i, v := range args {
		values[i] = reflect.ValueOf(v)
	}

	f.Call(values)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func makeAddressListConsumer(event string, c chan connectEvent) func([]structure.AddressConfiguration) {
	return func(list []structure.AddressConfiguration) {
		c <- connectEvent{event: event, addressList: list}
	}
}

func getOutboundIp() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.To4().String(), nil
}

func getModuleDeclaration(moduleInfo ModuleInfo) structure.BackendDeclaration {
	endpoints := backend.GetEndpoints(moduleInfo.ModuleName, moduleInfo.Handlers...)
	addr := moduleInfo.GrpcOuterAddress.IP
	hasSchema := strings.Contains(addr, "http://")
	if hasSchema {
		addr = strings.Replace(addr, "http://", "", -1)
	}
	if addr == "" {
		ip, err := getOutboundIp()
		if err != nil {
			panic(err)
		} else {
			if hasSchema {
				ip = fmt.Sprintf("http://%s", ip)
			}
			moduleInfo.GrpcOuterAddress.IP = ip
		}
	}
	return structure.BackendDeclaration{
		ModuleName: moduleInfo.ModuleName,
		Version:    moduleInfo.ModuleVersion,
		Address:    moduleInfo.GrpcOuterAddress,
		LibVersion: LibraryVersion,
		Endpoints:  endpoints,
	}
}

func getWsUrl(host string, port int, secure bool, params map[string]string) string {
	var prefix string
	if secure {
		prefix = "wss://"
	} else {
		prefix = "ws://"
	}
	etpUrl := "/isp-etp/"
	connectionString := prefix + host + ":" + strconv.Itoa(port) + etpUrl
	if len(params) > 0 {
		vals := url.Values{}
		for k, v := range params {
			vals.Add(k, v)
		}
		connectionString += "?" + vals.Encode()
	}
	return connectionString
}

func getConfigServiceConnectionStrings(sc structure.SocketConfiguration) ([]string, error) {
	hosts := strings.Split(sc.Host, ";")
	ports := strings.Split(sc.Port, ";")
	if len(hosts) != len(ports) {
		return nil, fmt.Errorf("different number of hosts/ports: %d/%d", len(hosts), len(ports))
	}
	connStrings := make([]string, len(hosts))
	for i := 0; i < len(hosts); i++ {
		port, err := strconv.Atoi(ports[i])
		if err != nil {
			return nil, err
		}
		connectionString := getWsUrl(
			hosts[i],
			port,
			sc.Secure,
			sc.UrlParams,
		)
		connStrings[i] = connectionString
	}
	return connStrings, nil
}

func ackEvent(client etp.Client, event string, data interface{}, bf backoff.BackOff) (bool, []byte, []byte) {
	bytes, err := json.Marshal(data)
	if err != nil {
		log.WithMetadata(log.Metadata{"event": event}).
			Errorf(stdcodes.ConfigServiceSendDataError, "marshal payload to json: %v", err)
		return false, nil, nil
	}

	var response []byte
	ack := func() error {
		response, err = client.EmitWithAck(context.Background(), event, bytes)
		if err != nil {
			return err
		} else if string(response) != utils.WsOkResponse {
			return errors.New("invalid response")
		}
		return nil
	}
	err = backoff.Retry(ack, bf)
	if err != nil {
		log.WithMetadata(log.Metadata{"event": event}).
			Errorf(stdcodes.ConfigServiceSendDataError, "ack event to config service: %v", err)
		return false, bytes, nil
	}

	return true, bytes, response
}

func getDefaultBackoff(ctx context.Context, maxElapsedTime time.Duration) backoff.BackOff {
	backOff := backoff.NewExponentialBackOff()
	backOff.MaxElapsedTime = maxElapsedTime
	bf := backoff.WithContext(backOff, ctx)
	return bf
}

type RoundRobinStrings struct {
	strings []string
	index   int
}

func (u *RoundRobinStrings) Get() string {
	if u.index == -1 {
		var random = rand.New(rand.NewSource(time.Now().UnixNano()))
		u.index = random.Intn(len(u.strings))
	} else {
		u.index += 1
		if u.index > len(u.strings)-1 {
			u.index = 0
		}
	}
	return u.strings[u.index]
}

func NewRoundRobinStrings(urls []string) *RoundRobinStrings {
	return &RoundRobinStrings{
		strings: urls,
		index:   -1,
	}
}
