package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	etp "github.com/integration-system/isp-etp-go/v2"
	"github.com/integration-system/isp-lib/v2/config/schema"
	"github.com/integration-system/isp-lib/v2/utils"
)

const (
	timeoutValidConnect = 3 * time.Second
	timeoutListen       = 700 * time.Millisecond
	timeoutDisconnect   = 100 * time.Millisecond
)

var _validRemoteConfig = RemoteConfig{Something: "Something text"}

func (cfg *bootstrapConfiguration) testRun(tb *testingBox) {
	tb.moduleRunner = makeRunner(*cfg)
	err := tb.moduleRunner.run()
	tb.testingFuncs.errorHandlingTestRun(err, tb.t)
}

func (tb *testingBox) testingServersRun() {
	ms := newMockServer()
	ms.subscribeAll(tb.handleServerFuncs)

	tb.tmpDir = setupConfig(tb.t, "127.0.0.1", ms.addr.Port)

	cfg := ServiceBootstrap(&Configuration{}, &RemoteConfig{}).
		DefaultRemoteConfigPath(schema.ResolveDefaultConfigPath(filepath.Join(tb.tmpDir, "/default_remote_config.json"))).
		SocketConfiguration(socketConfiguration).
		OnSocketErrorReceive(tb.moduleFuncs.onRemoteErrorReceive).
		DeclareMe(makeDeclaration).
		OnRemoteConfigReceive(tb.moduleFuncs.onRemoteConfigReceive)

	go cfg.testRun(tb)

	tb.testingFuncs.waitFullConnect(tb)
}

func (tb *testingBox) testingListener() {
	var index int
	timeOut := time.After(timeoutListen)
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
			} else if event.typeEvent != tb.expectedOrder[index] {
				tb.t.Errorf("order is broken, expected:\n%s\n got:\n%s", tb.expectedOrder[index], event.typeEvent)
			}
			if event.typeEvent == eventHandleConnect {
				tb.conn = event.conn
			}
			if event.err != nil {
				tb.t.Error(tb.errorHandling(event, index))
			}
			index++
			timeOut = time.After(timeoutListen)
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

func (tb *testingBox) reconnectModule() {
	if err := tb.conn.Close(); err != nil {
		tb.t.Error(err)
	}
	timeout := time.After(timeoutDisconnect)

	select {
	case <-timeout:
		tb.t.Errorf("Time to reconnect after disconnect is over: %v", timeoutDisconnect)
	case event := <-tb.checkingChan:
		if event.typeEvent != eventHandleDisconnect {
			tb.t.Errorf("Expected event %s got %s", eventHandleDisconnect, event.typeEvent)
		}
	}
	tb.testingFuncs.waitFullConnect(tb)
}

// Валидный тест, проверяет насколько подключение прошло успешно.
// В качестве проверяемых параметров используется количество и порядок событий.
func TestDefaultValid(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.testingServersRun()
	tb.testingListener()
	tb.reconnectModule()
	tb.testingListener()

}

// В этом тесте производим отправку невалидного конфига в обработчике handleConfigSchema
// Под невалидным понимается конфиг с иными полями
// При получении невалидного конфига модуль завершает свою работу
// фатальной ошибкой с описанием невалидных полей в конфигурации
func Test_moduleReceivedAnotherConfig(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)

	tb.expectedOrder = []eventType{
		eventHandleConnect,
		eventHandledConfigSchema,
	}
	tb.handleServerFuncs.handleConfigSchema = func(conn etp.Conn, data []byte) []byte {
		event := checkingEvent{typeEvent: eventHandledConfigSchema, conn: conn}
		defer func() {
			tb.checkingChan <- event
		}()
		if err := conn.Emit(context.Background(), utils.ConfigSendConfigWhenConnected,
			[]byte("{\"tomething\":\"Something text\"}")); err != nil {
			event.err = err
			return []byte(err.Error())
		}
		return []byte(utils.WsOkResponse)
	}
	tb.moduleFuncs.onRemoteConfigReceive = func(remoteConfig, _ *RemoteConfig) {
		tb.t.Error("received from mock config-service RemoteConfig did not cause of terminate the module")
	}
	tb.testingFuncs.errorHandlingTestRun = func(err error, t *testing.T) {
		if err == nil {
			t.Errorf("Expected errror from run method, but received none")
			return
		}
		if !strings.HasPrefix(err.Error(), "received invalid remote config:") {
			t.Errorf("Expected errror with prefix: \n\"received invalid remote config:\"\nbut got:\n%v", err)
		}
	}
	tb.testingFuncs.waitFullConnect = func(tb *testingBox) {
		timeout := time.After(timeoutValidConnect)
		select {
		case <-timeout:
		case <-tb.moduleReadyChan:
			tb.t.Errorf("Connect was established")
		}
	}

	tb.testingServersRun()
	tb.testingListener()
}

// WORK IN PROGRESS
// Проверяется положение: Если в процессе “рукопожатия” или после от isp-config-service в ответ возвращает не “ok”
// или сервис становится недоступным, то модуль начинает процесс инициализации с самого начала.
// Группа тестов проверяет поведение при получении отличающегося от utils.WsOkResponse ответа из хендлеров
// handleConfigSchema handleModuleRequirements, handleModuleReady обрабатывающих события вызыванные горутинами:
// go b.sendModuleConfigSchema(), go b.sendModuleRequirements(), go b.sendModuleReady().
// Если данные тесты возвращают ошибки, скорее всего не обрабатывается отлчитый от utils.WsOkResponse ответ,
// возвращенный соответствующей функцией ackEvent
func Test_NotOkResponse_handleModuleRequirements(t *testing.T) {
	tb := (&testingBox{}).setDefault(t)
	defaultMaxAckRetryTimeout = 2 * time.Second

	var startTime, zeroTime time.Time

	tb.handleServerFuncs.handleModuleRequirements = func(conn etp.Conn, data []byte) []byte {
		tb.checkingChan <- checkingEvent{typeEvent: eventHandleModuleRequirements, conn: conn}
		if startTime == zeroTime {
			startTime = time.Now()
		}
		if startTime.After(time.Now().Add(-defaultMaxAckRetryTimeout)) {
			fmt.Println("Not OK")
			return []byte("NOT OK")
		} else {
			fmt.Println("OK")
			return []byte(utils.WsOkResponse)
		}
	}

	tb.testingServersRun()
	tb.testingListener()
}
