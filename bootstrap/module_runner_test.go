package bootstrap

import (
	"context"
	json2 "encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	etp "github.com/integration-system/isp-etp-go/v2"
	"github.com/integration-system/isp-lib/v2/config/schema"
	"github.com/integration-system/isp-lib/v2/utils"
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

const (
	compliteValidConnect = 500 * time.Millisecond
	listenTimeout        = 700 * time.Millisecond
)

var (
	_validRemoteConfig   = RemoteConfig{Something: "Something text"}
	_invalidRemoteConfig = RemoteConfig{Something: "Broken Message"}
)

func (tb *testingBox) testingServersRun() {
	ms := newMockServer()
	ms.subscribeAll(tb.handleServerFuncs)

	tb.tmpDir = setupConfig(tb.t, "127.0.0.1", ms.addr.Port)

	go ServiceBootstrap(&Configuration{}, &RemoteConfig{}).
		DefaultRemoteConfigPath(schema.ResolveDefaultConfigPath(filepath.Join(tb.tmpDir, "/default_remote_config.json"))).
		//OnLocalConfigLoad(onLocalConfigLoad).
		SocketConfiguration(socketConfiguration).
		OnSocketErrorReceive(tb.moduleFuncs.onRemoteErrorReceive).
		//OnConfigErrorReceive(onRemoteConfigErrorReceive).
		DeclareMe(makeDeclaration).
		OnRemoteConfigReceive(tb.moduleFuncs.onRemoteConfigReceive).
		//OnShutdown(onShutdown).
		Run()
}

func (tb *testingBox) testingListener() {
	<-time.After(compliteValidConnect)

	var index int
	timeOut := time.After(listenTimeout)
LOOP:
	for {
		select {
		case event := <-tb.checkingChan:
			index++
			if index > len(tb.expectedOrder) {
				if event.conn != nil {
					tb.t.Errorf("%s(%s connID) at place %d overflows the expected events limit %d",
						event.typeEvent.string(), event.conn.ID(), index, len(tb.expectedOrder))
				} else {
					tb.t.Errorf("%s is exceed the expected number of events %d",
						event.typeEvent.string(), len(tb.expectedOrder))
				}
			}
			if event.typeEvent == eventHandleConnect {
				tb.conn = event.conn
			}
			if event.err != nil {
				tb.t.Error(tb.errorHandling(event, index))
			}
			timeOut = time.After(listenTimeout)
		case <-timeOut:
			if index < len(tb.expectedOrder) {
				for i := index; i < len(tb.expectedOrder); i++ {
					tb.t.Errorf("Expected event %s did't appear", tb.expectedOrder[i].string())
				}
			}
			break LOOP
		}
	}
	if index != len(tb.expectedOrder) {
		tb.t.Errorf("The number of events does not match: expected %d got %d", index, len(tb.expectedOrder))
	}
}

func (tb *testingBox) closeConnectModule() {
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
			tb.t.Errorf("Expected event %s got %s", eventHandleDisconnect.string(), event.typeEvent.string())
		}
	}
}

func TestDefaultValid(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.testingServersRun()
	tb.testingListener()
	tb.closeConnectModule()
	tb.testingListener()
	if err := os.RemoveAll(tb.tmpDir); err != nil {
		t.Error(err)
	}
}

func Test_handleModuleRequirements_NotOkResponse(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.handleServerFuncs.handleModuleRequirements = func(conn etp.Conn, data []byte) []byte {
		tb.checkingChan <- checkingEvent{typeEvent: eventHandleModuleRequirements, conn: conn}
		return []byte("NOT OK")
	}

	tb.testingServersRun()
	tb.testingListener()
	if err := os.RemoveAll(tb.tmpDir); err != nil {
		t.Error(err)
	}

	//if this test has an errors, may be it be resolved in module_runner.go: func (b *runner) sendModuleRequirements()
	// don't handled "not ok" answer from askEvent func
}

func Test_moduleReceivedAnotherConfig(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.handleServerFuncs.handleConfigSchema = func(conn etp.Conn, data []byte) []byte {
		fmt.Println("In eventHandledConfigSchema was sent different configSchema")
		ce := checkingEvent{typeEvent: eventHandledConfigSchema, conn: conn}

		jsonInvalidConfig, err := json2.Marshal(_invalidRemoteConfig)
		if err != nil {
			ce.err = err
			tb.checkingChan <- ce
			return []byte(err.Error())
		}
		if err := conn.Emit(context.Background(), utils.ConfigSendConfigWhenConnected, jsonInvalidConfig); err != nil {
			ce.err = err
			tb.checkingChan <- ce
			return []byte(err.Error())
		}
		tb.checkingChan <- ce
		return []byte(utils.WsOkResponse)
	}

	tb.testingServersRun()
	tb.testingListener()
	if err := os.RemoveAll(tb.tmpDir); err != nil {
		t.Error(err)
	}
	//TODO разобраться правильно ли реализован тест
	//а именно в каком месте должна производиться проверка на соответствие "родного" конфига и полученного от конфиг-сервиса
}
