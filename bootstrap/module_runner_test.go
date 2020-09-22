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
	"github.com/integration-system/isp-lib/v2/config/schema"
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

/*
[2020-04-27T16:47:50.183+03:00] [WARN ] [0078] [failed to heartbeat config service: failed to ping: failed to write control frame opPing: websocket closed: failed to wait for pong: context deadline exceeded] []
[2020-04-27T16:47:50.183+03:00] [ERROR] [0078] [disconnected from config service ws://10.250.9.40:9001/isp-etp/?instance_uuid=ec183598-746a-425f-b5e5-27b7fa8bc6af&module_name=oauth: failed to get reader: failed to read frame header: websocket closed: failed to wait for pong: context deadline exceeded] []
[2020-04-27T16:47:57.597+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:SEND_CONFIG_SCHEMA"]
[2020-04-27T16:47:58.252+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:SEND_REQUIREMENTS"]
[2020-04-27T16:47:59.098+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:SEND_CONFIG_SCHEMA"]
[2020-04-27T16:48:01.161+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:SEND_CONFIG_SCHEMA"]
[2020-04-27T16:48:01.835+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:SEND_REQUIREMENTS"]
[2020-04-27T16:48:03.793+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:SEND_REQUIREMENTS"]
[2020-04-27T16:48:05.375+03:00] [ERROR] [0077] [ack event to config service: failed to write msg: websocket closed: failed to wait for pong: context deadline exceeded] [event="MODULE:READY"]



[2020-04-29T13:05:10.161+03:00] [FATAL] [0031] [could not read local config: read local config file: Config File "config" Not Found in "[/home/max/Projects/!altarix/isp-lib/bootstrap/conf]"] []

*/

type Configuration struct {
	InstanceUuid         string
	ModuleName           string
	ConfigServiceAddress structure.AddressConfiguration
	GrpcOuterAddress     structure.AddressConfiguration
	GrpcInnerAddress     structure.AddressConfiguration
}

type RemoteConfig struct {
	Something string
}

const (
	//moduleConnectToMockServer = 500 * time.Millisecond // нужно выбрать таймауты
	compliteValidConnect = 500 * time.Millisecond
	//compliteValidConnect      = 1 * time.Second
)

const (
	eventHandleConnect eventType = iota + 1
	eventHandledConfigSchema
	eventHandleModuleRequirements
	eventHandleModuleReady
	eventHandleDisconnect
	eventRemoteConfigReceive
	eventRemoteConfigErrorReceive
)

var (
	_validRemoteConfig = RemoteConfig{Something: "Something text"}
)

type CheckingEvent struct {
	typeEvent eventType
	conn      etp.Conn
	err       error
	data      []byte
}

type testingBox struct {
	checkingChan      chan CheckingEvent
	moduleInsertFuncs ModuleInsertFuncs
	handleInsertFuncs HandleInsertFuncs
	t                 *testing.T
	order             []eventType
	tmpDir            string
	conn              etp.Conn
}

type CheckingChan chan CheckingEvent
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

type ModuleInsertFuncs struct {
	onRemoteConfigReceive func(remoteConfig, _ *RemoteConfig, c chan<- CheckingEvent)
	onRemoteErrorReceive  func(errorMessage map[string]interface{}, c chan<- CheckingEvent)
}

type ModuleFuncs struct {
	onRemoteConfigReceive func(remoteConfig, _ *RemoteConfig)
	onRemoteErrorReceive  func(errorMessage map[string]interface{})
}

type HandleInsertFuncs struct {
}

func (tb *testingBox) insertCheckinChanInModule() *ModuleFuncs {
	//TODO may be need check nils in mif
	return &ModuleFuncs{
		onRemoteConfigReceive: func(remoteConfig, _ *RemoteConfig) {
			tb.moduleInsertFuncs.onRemoteConfigReceive(remoteConfig, nil, tb.checkingChan)
		},
		onRemoteErrorReceive: func(errorMessage map[string]interface{}) {
			tb.moduleInsertFuncs.onRemoteErrorReceive(errorMessage, tb.checkingChan)
		},
	}
}

func (tb *testingBox) testingServersRun(mFuncs *ModuleFuncs) {
	ms := newMockServer(tb.checkingChan)

	ms.SubscribeAll()

	tb.tmpDir = setupConfig(tb.t, "127.0.0.1", ms.addr.Port)

	go ServiceBootstrap(&Configuration{}, &RemoteConfig{}).
		DefaultRemoteConfigPath(schema.ResolveDefaultConfigPath(filepath.Join(tb.tmpDir, "/default_remote_config.json"))).
		//OnLocalConfigLoad(onLocalConfigLoad).
		SocketConfiguration(socketConfiguration).
		OnSocketErrorReceive(mFuncs.onRemoteErrorReceive).
		OnConfigErrorReceive(onRemoteConfigErrorReceive).
		DeclareMe(makeDeclaration).
		OnRemoteConfigReceive(mFuncs.onRemoteConfigReceive).
		//OnShutdown(onShutdown).
		Run()
}

func (tb *testingBox) testingListner(errorHandling func(CheckingEvent, int) string) {
	//TODO решить нужно ли сначала принимать первый connect а потом ожидать
	<-time.After(compliteValidConnect)

	if len(tb.checkingChan) != len(tb.order) {
		tb.t.Errorf("The number of events %d does not match the expected %d", len(tb.checkingChan), len(tb.order))
	}

	//var event CheckingEvent
	for index, orderItem := range tb.order {
		event := <-tb.checkingChan
		if event.typeEvent != orderItem {
			tb.t.Errorf("The order is invalid in %d place \nexpected: %s\ngot: %s",
				index, event.typeEvent.String(), orderItem.String())
		}
		if event.typeEvent == eventHandleConnect {
			tb.conn = event.conn
		}
		if event.err != nil {
			tb.t.Error(errorHandling(event, index))
		}
	}
}

func (tb *testingBox) reconnectAndListenModule(errorHandling func(CheckingEvent, int) string) {
	if err := tb.conn.Close(); err != nil {
		tb.t.Error(err)
	}
	timeout := time.After(compliteValidConnect)

	select {
	case <-timeout:
		tb.t.Errorf("Time to reconnect after disconnect is over: %v", compliteValidConnect)
		return
	case event := <-tb.checkingChan:
		if event.typeEvent != eventHandleDisconnect {
			tb.t.Errorf("Expected event %s got %s", eventHandleDisconnect.String(), event.typeEvent.String())
		}
	}

	tb.testingListner(errorHandling)
}

func errorHandlingFor_ValidNewModule(event CheckingEvent, index int) string {
	str := fmt.Sprintf("ERROR: At order %d event %s happend\n", index, event.typeEvent.String())
	switch event.typeEvent {
	case eventRemoteConfigReceive:
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
		//case ...:
		//Handling other specific errors
	}
	return str
}

func TestBootstrapConfiguration_ValidNewModule(t *testing.T) {
	tb := testingBox{
		t:            t,
		checkingChan: make(CheckingChan, 20),
		moduleInsertFuncs: ModuleInsertFuncs{
			onRemoteConfigReceive: onRemoteConfigReceive,
			onRemoteErrorReceive:  onRemoteErrorReceive,
		},
		handleInsertFuncs: HandleInsertFuncs{}, // TODO вынести хендлеры в отдельную структуру
		order: []eventType{
			eventHandleConnect,
			eventHandledConfigSchema,
			eventRemoteConfigReceive,
			eventHandleModuleReady,
			eventHandleModuleRequirements,
		},
	}

	tb.testingServersRun(tb.insertCheckinChanInModule())

	tb.testingListner(errorHandlingFor_ValidNewModule)

	tb.reconnectAndListenModule(errorHandlingFor_ValidNewModule)

	if err := os.RemoveAll(tb.tmpDir); err != nil {
		t.Error(err)
	}

	return
}

func setupConfig(t *testing.T, configAddr, configPort string) string {
	//m := map[string]string{
	//	"LC_ISP_CONFIGSERVICEADDRESS.IP":   configAddr,
	//	"LC_ISP_CONFIGSERVICEADDRESS.PORT": configPort,
	//}
	//for k, v := range m {
	//	os.Setenv(k, v)
	//}

	viper.Reset()
	viper.SetEnvPrefix(config.LocalConfigEnvPrefix)
	viper.AutomaticEnv()
	viper.SetConfigName("config")
	//viper.ReadConfig()
	//viper.SetFs()

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

func onRemoteConfigReceive(remoteConfig, _ *RemoteConfig, c chan<- CheckingEvent) {
	fmt.Printf("-4onRemote ConfigReceive\n")

	ce := CheckingEvent{typeEvent: eventRemoteConfigReceive}
	if *remoteConfig == _validRemoteConfig {
		c <- ce
	} else {
		jsonConfig, err := json2.Marshal(remoteConfig)
		if err != nil {
			ce.err = errors.New("Can't Marshal handled remoteConfig")
		} else {
			ce.err = errors.New("Received from mock RemoteConfig is not matches with original")
			ce.data = jsonConfig
		}
		c <- ce
	}
}

func onRemoteErrorReceive(errorMessage map[string]interface{}, c chan<- CheckingEvent) {
	fmt.Printf("-onRemote ErrorReceive\n")
	c <- CheckingEvent{typeEvent: eventRemoteConfigErrorReceive}
	log.WithMetadata(errorMessage).Error(stdcodes.ReceiveErrorFromConfig, "error from config service")
}

func onRemoteConfigErrorReceive(errorMessage string) {
	fmt.Printf("-onRemote ConfigErrorReceive\n")
	log.WithMetadata(map[string]interface{}{
		"message": errorMessage,
	}).Error(stdcodes.ReceiveErrorOnGettingConfigFromConfig, "error on getting remote configuration")
}

// ---------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------
//                          MockConfigServer
// ---------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------
// ---------------------------------------------------------------------------------

type mockConfigServer struct {
	etpServer    etp.Server
	httpServer   *http.Server
	addr         structure.AddressConfiguration
	checkingChan CheckingChan
}

func newMockServer(cc CheckingChan) *mockConfigServer {
	srv := &mockConfigServer{checkingChan: cc}

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

func (s *mockConfigServer) SubscribeAll() {
	s.etpServer.
		OnConnect(s.handleConnect).
		OnDisconnect(s.handleDisconnect).
		//OnError(s.handleError). //придумать что с этим делать
		OnWithAck(utils.ModuleReady, s.handleModuleReady).
		OnWithAck(utils.ModuleSendRequirements, s.handleModuleRequirements).
		OnWithAck(utils.ModuleSendConfigSchema, s.handleConfigSchema)
}

//
func (s *mockConfigServer) Address() structure.AddressConfiguration {
	return s.addr
}

func (h *mockConfigServer) handleConnect(conn etp.Conn) {
	fmt.Printf("-1handled Connect: %v\n", conn.ID())
	h.checkingChan <- CheckingEvent{typeEvent: eventHandleConnect, conn: conn}
}

func (h *mockConfigServer) handleDisconnect(conn etp.Conn, _ error) {

	fmt.Printf("-handled Disconnect: %v\n", conn.ID())

	h.checkingChan <- CheckingEvent{typeEvent: eventHandleDisconnect}
}

func (h *mockConfigServer) handleModuleReady(conn etp.Conn, data []byte) []byte {
	fmt.Printf("-6handled ModuleReady: %v\n", conn.ID())

	h.checkingChan <- CheckingEvent{typeEvent: eventHandleModuleReady}

	log.Debugf(0, "handleModuleReady moduleName: %s", "test")
	return []byte(utils.WsOkResponse)
}

func (h *mockConfigServer) handleModuleRequirements(conn etp.Conn, data []byte) []byte {
	fmt.Printf("-5handled ModuleRequirements: %v\n", conn.ID())

	h.checkingChan <- CheckingEvent{typeEvent: eventHandleModuleRequirements}

	return []byte(utils.WsOkResponse)
}

func (h *mockConfigServer) handleConfigSchema(conn etp.Conn, data []byte) []byte {
	//fmt.Printf("-2handled ConfigSchema: %v\n", conn.ID())

	h.checkingChan <- CheckingEvent{typeEvent: eventHandledConfigSchema}

	type confSchema struct {
		Config json2.RawMessage
	}

	moduleName := "test"
	log.Debugf(0, "handleConfigSchema moduleName: %s", moduleName)

	var configSchema confSchema
	if err := json.Unmarshal(data, &configSchema); err != nil {
		return []byte(err.Error())
	}
	conn.Emit(context.Background(), utils.ConfigSendConfigWhenConnected, configSchema.Config)

	return []byte(utils.WsOkResponse)
}
