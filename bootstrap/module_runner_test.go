package bootstrap

import (
	"context"
	json2 "encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	etp "github.com/integration-system/isp-etp-go/v2"
	"github.com/integration-system/isp-lib/v2/config/schema"
	"github.com/integration-system/isp-lib/v2/utils"
)

const (
	completeValidConnect = 100 * time.Millisecond
	listenTimeout        = 700 * time.Millisecond
)

var _validRemoteConfig = RemoteConfig{Something: "Something text"}

func (cfg *bootstrapConfiguration) testRun(c chan<- *runner, tb *testingBox) {
	runner := makeRunner(*cfg)
	c <- runner
	err := runner.run()
	tb.handleTestingErrorsFuncs.errorHandlingTestRun(err, tb.t)
}

func (tb *testingBox) testingServersRun() {
	ms := newMockServer()
	ms.subscribeAll(tb.handleServerFuncs)

	tb.tmpDir = setupConfig(tb.t, "127.0.0.1", ms.addr.Port)
	runnerChan := make(chan *runner)

	cfg := ServiceBootstrap(&Configuration{}, &RemoteConfig{}).
		DefaultRemoteConfigPath(schema.ResolveDefaultConfigPath(filepath.Join(tb.tmpDir, "/default_remote_config.json"))).
		//OnLocalConfigLoad(onLocalConfigLoad).
		SocketConfiguration(socketConfiguration).
		OnSocketErrorReceive(tb.moduleFuncs.onRemoteErrorReceive).
		//OnConfigErrorReceive(onRemoteConfigErrorReceive).
		DeclareMe(makeDeclaration).
		OnRemoteConfigReceive(tb.moduleFuncs.onRemoteConfigReceive)
	//OnShutdown(onShutdown).
	go cfg.testRun(runnerChan, tb)
	tb.moduleRunner = <-runnerChan
}

func (tb *testingBox) testingListener() {
	<-time.After(completeValidConnect)

	var index int
	timeOut := time.After(listenTimeout)
LOOP:
	for {
		select {
		case event := <-tb.checkingChan:
			if index > len(tb.expectedOrder)-1 {
				if event.conn != nil {
					tb.t.Errorf("%s(%s connID) at place %d overflows the expected events limit %d",
						event.typeEvent, event.conn.ID(), index+1, len(tb.expectedOrder))
				} else {
					tb.t.Errorf("%s is exceed the expected number of events %d",
						event.typeEvent, len(tb.expectedOrder))
				}

			} else if event.typeEvent != tb.expectedOrder[index] { // TODO найти решение для hard и soft списков
				tb.t.Errorf("order is broken, expected:\n%s\n got:\n%s", tb.expectedOrder[index], event.typeEvent)
			}
			if event.typeEvent == eventHandleConnect {
				tb.conn = event.conn
			}
			if event.err != nil {
				tb.t.Error(tb.errorHandling(event, index))
			}
			index++
			timeOut = time.After(listenTimeout)
		case <-timeOut:
			if index < len(tb.expectedOrder) {
				for i := index; i < len(tb.expectedOrder); i++ {
					tb.t.Errorf("Expected event %s did't appear", tb.expectedOrder[i])
				}
			}
			break LOOP
		}
	}
	if index != len(tb.expectedOrder) {
		tb.t.Errorf("The number of events does not match: expected %d got %d", len(tb.expectedOrder), index)
	}
}

func (tb *testingBox) closeConnectModule() {
	if err := tb.conn.Close(); err != nil {
		tb.t.Error(err)
	}
	timeout := time.After(completeValidConnect)

	select {
	case <-timeout:
		tb.t.Errorf("Time to reconnect after disconnect is over: %v", completeValidConnect)
		return
	case event := <-tb.checkingChan:
		if event.typeEvent != eventHandleDisconnect {
			tb.t.Errorf("Expected event %s got %s", eventHandleDisconnect, event.typeEvent)
		}
	}
}

//	Валидный тест, проверяет насколько подключение прошло успешно
//	В качестве проверяемых параметров используется количество и порядок событий
func TestDefaultValid(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.testingServersRun()
	tb.testingListener()
	tb.closeConnectModule()
	tb.testingListener()

}

//	Проверяется положение: Если в процессе “рукопожатия” или после от isp-config-service в ответ возвращает не “ok”
//или сервис становится недоступным, то модуль начинает процесс инициализации с самого начала.
//	Группа тестов проверяет поведение при получении отличающегося от utils.WsOkResponse ответа из хендлеров
//handleConfigSchema handleModuleRequirements, handleModuleReady обрабатывающих события вызыванные горутинами
//go b.sendModuleConfigSchema(), go b.sendModuleRequirements(), go b.sendModuleReady()
func Test_handleModuleRequirements_NotOkResponse(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.handleServerFuncs.handleModuleRequirements = func(conn etp.Conn, data []byte) []byte {
		tb.checkingChan <- checkingEvent{typeEvent: eventHandleModuleRequirements, conn: conn}
		return []byte("NOT OK")
	}

	tb.testingServersRun()
	tb.testingListener()

	//if this test has an errors, may be it be resolved in module_runner.go: func (b *runner) sendModuleRequirements()
	// don't handled "not ok" answer from askEvent func
}

//	В этом тесте производим отправку невалидного конфига в обработчике handleConfigSchema
//	Под невалидным понимается конфиг с иными полями
//	При получении невалидного конфига модуль завершает свою работу с фатальной ошибкой с описанием невалидных полей в конфигурации
func Test_moduleReceivedAnotherConfig(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.expectedOrder = []eventType{
		eventHandleConnect,
		eventHandledConfigSchema,
	}
	tb.handleServerFuncs.handleConfigSchema = func(conn etp.Conn, data []byte) []byte {
		fmt.Println("In eventHandledConfigSchema was sent different configSchema")
		event := checkingEvent{typeEvent: eventHandledConfigSchema, conn: conn}
		defer func() {
			tb.checkingChan <- event
		}()

		type confSchema struct {
			Config json2.RawMessage
		}
		var configSchema confSchema
		if err := json.Unmarshal(data, &configSchema); err != nil {
			event.err = err
			return []byte(err.Error())
		}
		configSchema.Config[2]++
		if err := conn.Emit(context.Background(), utils.ConfigSendConfigWhenConnected, configSchema.Config); err != nil {
			event.err = err
			return []byte(err.Error())
		}
		return []byte(utils.WsOkResponse)
	}
	tb.moduleFuncs.onRemoteConfigReceive = func(remoteConfig, _ *RemoteConfig) {
		event := checkingEvent{typeEvent: eventRemoteConfigReceive}
		if *remoteConfig == _validRemoteConfig {
			jsonConfig, _ := json2.Marshal(remoteConfig)
			event.err = errors.New("received from mock RemoteConfig is not INVALID, witch expected")
			event.data = jsonConfig
		}
		tb.checkingChan <- event
	}
	tb.handleTestingErrorsFuncs.errorHandlingTestRun = func(err error, t *testing.T) {
		if err == nil {
			t.Errorf("Expected errror from run method, but received none")
			return
		}
		jsonConfig, _ := json2.Marshal(RemoteConfig{Something: "Something text"})
		jsonConfig[2] = jsonConfig[2] + 33
		expectedErr := fmt.Errorf("received invalid remote config: Something -> Required, config=%v", string(jsonConfig))
		if err.Error() != expectedErr.Error() {
			t.Errorf("Expected errror: \n%v\n but got: \n%v", expectedErr, err)
		}
	}

	tb.testingServersRun()
	tb.testingListener()
}
