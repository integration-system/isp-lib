package bootstrap

import (
	"context"
	json2 "encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	etp "github.com/integration-system/isp-etp-go/v2"
	"github.com/integration-system/isp-lib/v2/config"
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type Configuration struct {
	InstanceUuid         string
	ModuleName           string
	ConfigServiceAddress structure.AddressConfiguration
	GrpcOuterAddress     structure.AddressConfiguration
	GrpcInnerAddress     structure.AddressConfiguration
}

type RemoteConfig struct {
	Something string `valid:"required~Required"`
}

type mockConfigServer struct {
	etpServer  etp.Server
	httpServer *http.Server
	addr       structure.AddressConfiguration
}

const (
	eventHandleConnect eventType = iota + 1
	eventHandledConfigSchema
	eventHandleModuleRequirements
	eventHandleModuleReady
	eventHandleDisconnect
	eventRemoteConfigReceive
	eventRemoteConfigErrorReceive
)

type eventType uint

func (et eventType) String() string {
	switch et {
	case eventHandleConnect:
		return "eventHandleConnect"
	case eventHandledConfigSchema:
		return "eventHandledConfigSchema"
	case eventHandleModuleRequirements:
		return "eventHandleModuleRequirements"
	case eventHandleModuleReady:
		return "eventHandleModuleReady"
	case eventHandleDisconnect:
		return "eventHandleDisconnect"
	case eventRemoteConfigReceive:
		return "eventRemoteConfigReceive"
	case eventRemoteConfigErrorReceive:
		return "eventRemoteConfigErrorReceive"
	default:
		return "(ERROR: Can't find type of event)"
	}
}

type checkingEvent struct {
	typeEvent eventType
	conn      etp.Conn
	err       error
	data      []byte
}

type testingBox struct {
	checkingChan      chan checkingEvent
	moduleReadyChan   chan checkingEvent
	moduleFuncs       moduleFuncs
	handleServerFuncs handleServerFuncs
	testingFuncs      testingFuncs
	t                 *testing.T
	expectedOrder     []eventType
	tmpDir            string
	conn              etp.Conn
	moduleRunner      *runner
}

type moduleFuncs struct {
	onRemoteConfigReceive func(remoteConfig, _ *RemoteConfig)
	onRemoteErrorReceive  func(errorMessage map[string]interface{})
}

type handleServerFuncs struct {
	handleConnect            func(conn etp.Conn)
	handleDisconnect         func(conn etp.Conn, _ error)
	handleModuleReady        func(conn etp.Conn, data []byte) []byte
	handleModuleRequirements func(conn etp.Conn, data []byte) []byte
	handleConfigSchema       func(conn etp.Conn, data []byte) []byte
	handleTestingEvent       func(conn etp.Conn, data []byte) []byte
}

type testingFuncs struct {
	errorRemoteConfigReceive func(event checkingEvent, str string) string
	errorHandledConfigSchema func(event checkingEvent, str string) string
	errorHandlingTestRun     func(err error, t *testing.T)
	waitFullConnect          func(tb *testingBox)
}

func (tb *testingBox) setDefault(t *testing.T) *testingBox {
	defaultMaxAckRetryTimeout = 10 * time.Second

	tb.t = t
	tb.checkingChan = make(chan checkingEvent, 20)
	tb.moduleReadyChan = make(chan checkingEvent)
	tb.expectedOrder = []eventType{
		eventHandleConnect,
		eventHandledConfigSchema,
		eventRemoteConfigReceive,
		eventHandleModuleRequirements,
		eventHandleModuleReady,
	}

	tb.moduleFuncs.setDefault(tb.checkingChan)
	tb.handleServerFuncs.setDefault(tb.checkingChan, tb.moduleReadyChan)
	tb.testingFuncs.setDefault()

	return tb
}

func (m *moduleFuncs) setDefault(checkingChan chan<- checkingEvent) {
	m.onRemoteConfigReceive = func(remoteConfig, _ *RemoteConfig) {
		event := checkingEvent{typeEvent: eventRemoteConfigReceive}
		if *remoteConfig != _validRemoteConfig {
			jsonConfig, err := json2.Marshal(remoteConfig)
			if err != nil {
				event.err = errors.New("can't Marshal handled remoteConfig")
			} else {
				event.err = errors.New("received from mock RemoteConfig is not matches with _validRemoteConfig")
				event.data = jsonConfig
			}
		}
		checkingChan <- event
	}
	m.onRemoteErrorReceive = func(errorMessage map[string]interface{}) {
		checkingChan <- checkingEvent{typeEvent: eventRemoteConfigErrorReceive}
	}
}

func (h *handleServerFuncs) setDefault(checkingChan, moduleReadyChan chan<- checkingEvent) {
	h.handleConnect = func(conn etp.Conn) {
		checkingChan <- checkingEvent{typeEvent: eventHandleConnect, conn: conn}
	}
	h.handleDisconnect = func(conn etp.Conn, _ error) {
		checkingChan <- checkingEvent{typeEvent: eventHandleDisconnect, conn: conn}
	}
	h.handleModuleReady = func(conn etp.Conn, data []byte) []byte {
		event := checkingEvent{typeEvent: eventHandleModuleReady, conn: conn}
		checkingChan <- event
		moduleReadyChan <- event
		return []byte(utils.WsOkResponse)
	}
	h.handleModuleRequirements = func(conn etp.Conn, data []byte) []byte {
		checkingChan <- checkingEvent{typeEvent: eventHandleModuleRequirements, conn: conn}
		return []byte(utils.WsOkResponse)
	}
	h.handleConfigSchema = func(conn etp.Conn, data []byte) []byte {
		event := checkingEvent{typeEvent: eventHandledConfigSchema, conn: conn}
		defer func() {
			checkingChan <- event
		}()

		type confSchema struct {
			Config json2.RawMessage
		}
		var configSchema confSchema
		if err := json.Unmarshal(data, &configSchema); err != nil {
			event.err = err
			return []byte(err.Error())
		}
		if err := conn.Emit(context.Background(), utils.ConfigSendConfigWhenConnected, configSchema.Config); err != nil {
			event.err = err
			return []byte(err.Error())
		}
		return []byte(utils.WsOkResponse)
	}
}

func (t *testingFuncs) setDefault() {
	t.errorRemoteConfigReceive = func(event checkingEvent, str string) string {
		str = fmt.Sprintf("%s%s\n", str, event.err)
		if len(event.data) != 0 {
			var dataUnmarsh RemoteConfig
			err := json2.Unmarshal(event.data, &dataUnmarsh)
			if err != nil {
				str = fmt.Sprintf("%s%s\n", str, err)
			} else {
				str = fmt.Sprintf("%s%v\n", str, dataUnmarsh)
			}
		}
		return str
	}
	t.errorHandledConfigSchema = func(event checkingEvent, str string) string {
		return fmt.Sprintf("%s %s", str, event.err)
	}
	t.errorHandlingTestRun = func(err error, t *testing.T) {
		if err != nil {
			t.Errorf("run method was stopped by error: %v", err)
		}
	}
	t.waitFullConnect = func(tb *testingBox) {
		timeout := time.After(timeoutValidConnect)
		select {
		case <-timeout:
			tb.t.Errorf("Waiting time %s for full connect module is over", timeoutValidConnect)
		case <-tb.moduleReadyChan:
		}
	}
}

func (tb *testingBox) errorHandling(event checkingEvent, index int) string {
	str := fmt.Sprintf("ERROR: At order %d was happend %s\n", index, event.typeEvent)
	switch event.typeEvent {
	case eventRemoteConfigReceive:
		return tb.testingFuncs.errorRemoteConfigReceive(event, str)
	case eventHandledConfigSchema:
		return tb.testingFuncs.errorHandledConfigSchema(event, str)
	}
	return str
}

func newMockServer() *mockConfigServer {
	srv := &mockConfigServer{}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	srv.addr = structure.AddressConfiguration{
		IP:   "",
		Port: strings.Split(listener.Addr().String(), ":")[1],
	}

	etpConfig := etp.ServerConfig{
		InsecureSkipVerify: true,
	}
	srv.etpServer = etp.NewServer(context.Background(), etpConfig)
	mux := http.NewServeMux()
	mux.HandleFunc("/isp-etp/", srv.etpServer.ServeHttp)
	srv.httpServer = &http.Server{Handler: mux}
	go func() {
		if err := srv.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf(0, "http server closed: %v", err)
		}
	}()

	return srv
}

func (s *mockConfigServer) subscribeAll(th handleServerFuncs) {
	s.etpServer.
		OnConnect(th.handleConnect).
		OnDisconnect(th.handleDisconnect).
		OnWithAck(utils.ModuleReady, th.handleModuleReady).
		OnWithAck(utils.ModuleSendRequirements, th.handleModuleRequirements).
		OnWithAck(utils.ModuleSendConfigSchema, th.handleConfigSchema)
}

func setupConfig(t *testing.T, configAddr, configPort string) string {
	viper.Reset()
	viper.SetEnvPrefix(config.LocalConfigEnvPrefix)
	viper.AutomaticEnv()
	viper.SetConfigName("config")

	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		panic(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})
	viper.AddConfigPath(tmpDir)

	conf := Configuration{
		InstanceUuid: "",
		ModuleName:   "test",
		ConfigServiceAddress: structure.AddressConfiguration{
			Port: configPort,
			IP:   configAddr,
		},
		GrpcOuterAddress: structure.AddressConfiguration{
			Port: "9371",
			IP:   "127.0.0.1",
		},
		GrpcInnerAddress: structure.AddressConfiguration{},
	}

	bytes, err := yaml.Marshal(conf)
	if err != nil {
		panic(err)
	}

	configFile := filepath.Join(tmpDir, "config.yml")
	if err := ioutil.WriteFile(configFile, bytes, 0666); err != nil {
		panic(err)
	}

	bytes, err = json.Marshal(_validRemoteConfig)
	if err != nil {
		panic(err)
	}

	remoteConfigFile := filepath.Join(tmpDir, "default_remote_config.json")
	if err := ioutil.WriteFile(remoteConfigFile, bytes, 0666); err != nil {
		panic(err)
	}

	return tmpDir
}

func makeDeclaration(localConfig interface{}) ModuleInfo {
	cfg := localConfig.(*Configuration)
	return ModuleInfo{
		ModuleName:       cfg.ModuleName,
		ModuleVersion:    "vtest",
		GrpcOuterAddress: cfg.GrpcOuterAddress,
		Endpoints:        []structure.EndpointDescriptor{},
	}
}

func socketConfiguration(cfg interface{}) structure.SocketConfiguration {
	appConfig := cfg.(*Configuration)
	return structure.SocketConfiguration{
		Host:   appConfig.ConfigServiceAddress.IP,
		Port:   appConfig.ConfigServiceAddress.Port,
		Secure: false,
		UrlParams: map[string]string{
			"module_name":   appConfig.ModuleName,
			"instance_uuid": appConfig.InstanceUuid,
		},
	}
}
