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
	completeValidConnect = 500 * time.Millisecond
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
	<-time.After(completeValidConnect)

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
	timeout := time.After(completeValidConnect)

	select {
	case <-timeout:
		tb.t.Errorf("Time to reconnect after disconnect is over: %v", completeValidConnect)
		return
	case event := <-tb.checkingChan:
		if event.typeEvent != eventHandleDisconnect {
			tb.t.Errorf("Expected event %s got %s", eventHandleDisconnect.string(), event.typeEvent.string())
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

	tb.handleServerFuncs.handleConfigSchema = func(conn etp.Conn, data []byte) []byte {
		fmt.Println("In eventHandledConfigSchema was sent different configSchema")
		ce := checkingEvent{typeEvent: eventHandledConfigSchema, conn: conn}
		defer func() {
			tb.checkingChan <- ce
		}()

		jsonInvalidConfig, err := json2.Marshal(_invalidRemoteConfig)
		if err != nil {
			ce.err = err
			return []byte(err.Error())
		}
		if err := conn.Emit(context.Background(), utils.ConfigSendConfigWhenConnected, jsonInvalidConfig); err != nil {
			ce.err = err
			return []byte(err.Error())
		}
		return []byte(utils.WsOkResponse)
	}

	tb.testingServersRun()
	tb.testingListener()

	//TODO разобраться правильно ли реализован тест
	//а именно в каком месте должна производиться проверка на соответствие "родного" конфига и полученного от конфиг-сервиса
}
