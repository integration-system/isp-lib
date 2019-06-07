package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/integration-system/golang-socketio"
	"github.com/integration-system/isp-lib/config"
	"github.com/integration-system/isp-lib/config/schema"
	"github.com/integration-system/isp-lib/logger"
	"github.com/integration-system/isp-lib/metric"
	"github.com/integration-system/isp-lib/socket"
	"github.com/integration-system/isp-lib/structure"
	"github.com/integration-system/isp-lib/utils"
	"github.com/mohae/deepcopy"
	"github.com/thecodeteam/goodbye"
	"os"
	"strings"
	"time"
)

type runner struct {
	bootstrapConfiguration

	moduleInfo ModuleInfo

	remoteConfigChan chan string
	routesChan       chan structure.RoutingConfig
	connectEventChan chan connectEvent
	exitChan         chan struct{}
	disconnectChan   chan struct{}

	client                   *gosocketio.Client
	ready                    bool
	lastFailedConnectionTime time.Time
}

func (b *runner) run() {
	ctx := b.initShutdownHandler()
	defer goodbye.Exit(ctx, 0)

	defer func() {
		err := recover()
		if err != nil {
			logger.Fatal(err)
		}
	}()

	b.initLocalConfig()      //read local configuration, calls callback
	b.initModuleInfo()       //set moduleInfo
	b.initSocketConnection() //create socket.io object, subscribe to all events
	b.initStatusMetrics()    //add socket and required modules connections checkers in metrics

	if b.declaratorAcquirer != nil {
		b.declaratorAcquirer(&declarator{b.sendModuleDeclaration}) //provides module declarator to clients code
	}

	b.sendModuleConfigSchema() //create and send schema with default remote config

	b.ready = false //module not ready state by default

	remoteConfigReady, requiredModulesReady, routesReady, currentConnectedModules := b.initialState()
	remoteConfigTimeoutChan := time.After(3 * time.Second) //used for log WARN message
	neverTriggerChan := make(chan time.Time)               //used for stops log flood
	initChan := make(chan struct{}, 1)
	//in main goroutine handle all asynchronous events from config service
	for {
		//if all conditions are true, put signal into channel and later in loop send MODULE:READY event to config-service
		if !b.ready && remoteConfigReady && requiredModulesReady && routesReady {
			initChan <- struct{}{}
		}

		select {
		case data := <-b.remoteConfigChan:
			oldConfigCopy := deepcopy.Copy(b.remoteConfigPtr)
			newRemoteConfig := config.InitRemoteConfig(oldConfigCopy, data)
			oldRemoteConfig := b.remoteConfigPtr
			if b.onRemoteConfigReceive != nil {
				callFunc(b.onRemoteConfigReceive, newRemoteConfig, oldRemoteConfig)
			}
			b.remoteConfigPtr = newRemoteConfig

			remoteConfigReady = true
			if !b.ready {
				b.sendModuleRequirements() //after first time receiving config, send requirements
			}

			remoteConfigTimeoutChan = neverTriggerChan //stop flooding in logs
		case <-remoteConfigTimeoutChan:
			logger.Warn("Remote config isn't received")
			remoteConfigTimeoutChan = time.After(3 * time.Second)
		case routers := <-b.routesChan:
			if b.onRoutesReceive != nil {
				routesReady = b.onRoutesReceive(routers)
			}
		case e := <-b.connectEventChan:
			if c, ok := b.requiredModules[e.event]; ok {
				if ok := c.consumer(e.addressList); ok {
					currentConnectedModules[e.event] = true
				}

				ok := true
				for e, consumer := range b.requiredModules {
					val := currentConnectedModules[e]
					if !val && consumer.mustConnect {
						ok = false
						break
					}
				}
				requiredModulesReady = ok

				addrList := make([]string, 0, len(e.addressList))
				if currentConnectedModules[e.event] {
					for _, addr := range e.addressList {
						addrList = append(addrList, addr.GetAddress())
					}
				}
				b.connectedModules[e.event] = addrList
			}
		case <-initChan:
			b.ready = true
			b.sendModuleReady()
		case <-b.disconnectChan: //on disconnection, set state to 'not ready' once again
			b.ready = false
			remoteConfigReady, requiredModulesReady, routesReady, currentConnectedModules = b.initialState()
		case <-b.exitChan: //return from main goroutine after shutdown signal
			return
		}

	}
}

func (b *runner) initShutdownHandler() context.Context {
	ctx := context.Background()

	goodbye.Notify(ctx)
	goodbye.Register(func(ctx context.Context, sig os.Signal) {
		logger.Info(logger.FmtAlertMsg("module shutting down now"))

		if b.client != nil {
			b.client.Close()
		}

		if b.onShutdown != nil {
			b.onShutdown(ctx, sig)
		}

		logger.Info(logger.FmtAlertMsg("module has gracefully shut down"))

		close(b.exitChan)
	})

	return ctx
}

func (b *runner) initLocalConfig() {
	if b.onLocalConfigChange != nil {
		config.OnConfigChange(b.onLocalConfigChange)
	}
	b.localConfigPtr = config.InitConfigV2(b.localConfigPtr, false)
	if b.onLocalConfigLoad != nil {
		callFunc(b.onLocalConfigLoad, b.localConfigPtr)
	}
}

func (b *runner) initModuleInfo() {
	b.moduleInfo = b.makeModuleInfo(config.Get())
}

func (b *runner) initSocketConnection() {
	if b.makeSocketConfig == nil {
		logger.Fatal("Socket configuration is not specified. Call 'SocketConfiguration' first")
		return
	}

	socketConfig := b.makeSocketConfig(b.localConfigPtr)
	builder := gosocketio.NewClientBuilder().
		EnableReconnection().
		ReconnectionTimeout(3*time.Second).
		OnReconnectionError(func(err error) {
			logger.Warnf("SocketIO reconnection error: %v", err)
			b.lastFailedConnectionTime = time.Now()
		}).
		On(gosocketio.OnDisconnection, func(arg interface{}) error {
			logger.Warn("SocketIO disconnected")
			b.lastFailedConnectionTime = time.Now()
			b.disconnectChan <- struct{}{}
			return nil
		}, nil)
	connectionString := socket.GetConnectionString(socketConfig)
	c := builder.BuildToConnect(connectionString)

	if b.onSocketErrorReceive != nil {
		must(c.On(utils.ErrorConnection, handleError(b.onSocketErrorReceive, utils.ErrorConnection)))
	}
	if b.onConfigErrorReceive != nil {
		must(c.On(utils.ConfigError, handleConfigError(b.onConfigErrorReceive, utils.ConfigError)))
	}
	if b.remoteConfigPtr != nil {
		must(c.On(utils.ConfigSendConfigWhenConnected, handleRemoteConfiguration(b.remoteConfigChan, utils.ConfigSendConfigWhenConnected)))
		must(c.On(utils.ConfigSendConfigChanged, handleRemoteConfiguration(b.remoteConfigChan, utils.ConfigSendConfigChanged)))
		must(c.On(utils.ConfigSendConfigOnRequest, handleRemoteConfiguration(b.remoteConfigChan, utils.ConfigSendConfigOnRequest)))
	}
	if b.onRoutesReceive != nil {
		must(c.On(utils.ConfigSendRoutesChanged, handleRoutes(b.routesChan, utils.ConfigSendRoutesChanged)))
		must(c.On(utils.ConfigSendRoutesWhenConnected, handleRoutes(b.routesChan, utils.ConfigSendRoutesWhenConnected)))
		must(c.On(utils.ConfigSendRoutesOnRequest, handleRoutes(b.routesChan, utils.ConfigSendRoutesOnRequest)))
	}
	for e := range b.requiredModules {
		must(c.On(e, UnmarshalAddressListAndThen(e, makeAddressListConsumer(e, b.connectEventChan))))
	}
	for e, f := range b.subs {
		must(c.On(e, f))
	}

	err := c.Dial()
	for err != nil {
		logger.Warnf("Could not connect to SocketIO: %v", err)
		b.lastFailedConnectionTime = time.Now()

		select {
		case <-b.exitChan:
			return
		case <-time.After(3 * time.Second):

		}
		err = c.Dial()
	}

	b.client = c
}

func (b *runner) initStatusMetrics() {
	metric.InitStatusChecker("config-websocket", func() interface{} {
		socketConfig := b.makeSocketConfig(b.localConfigPtr)
		uri := fmt.Sprintf("%s:%s", socketConfig.Host, socketConfig.Port)
		status := true
		if b.client == nil || !b.client.IsAlive() {
			status = false
		}
		lastFailedConnectionMsAgo := time.Duration(0)
		if !b.lastFailedConnectionTime.IsZero() {
			lastFailedConnectionMsAgo = time.Now().Sub(b.lastFailedConnectionTime) / 1e6
		}
		return map[string]interface{}{
			"connected":                 status,
			"lastFailedConnectionMsAgo": lastFailedConnectionMsAgo,
			"address":                   uri,
			"moduleReady":               b.ready,
		}
	})

	for k := range b.requiredModules {
		moduleName := strings.Replace(k, "_"+utils.ModuleConnectionSuffix, "", -1)
		keyCopy := k
		metric.InitStatusChecker(fmt.Sprintf("%s-grpc", moduleName), func() interface{} {
			addrList, ok := b.connectedModules[keyCopy]
			if ok {
				return addrList
			} else {
				return []string{}
			}
		})
	}
}

func (b *runner) sendModuleRequirements() {
	requiredModules := make([]string, 0, len(b.requiredModules))
	for evt := range b.requiredModules {
		requiredModules = append(requiredModules, evt)
	}

	requirements := ModuleRequirements{
		RequiredModules: requiredModules,
		RequireRoutes:   b.onRoutesReceive != nil,
	}

	if !requirements.IsEmpty() {
		logger.Infof("%s: %v", utils.ModuleSendRequirements, requirements)
		if s, err := json.Marshal(requirements); err != nil {
			logger.Warn("Could not serialize requirements to JSON", err)
		} else if err := b.client.Emit(utils.ModuleSendRequirements, string(s)); err != nil {
			logger.Warn("Could not send requirements", err)
		}
	}
}

func (b *runner) sendModuleDeclaration(eventType string) {
	b.moduleInfo = b.makeModuleInfo(b.localConfigPtr)

	bytes, err := getJsonModuleDeclaration(b.moduleInfo)
	if err != nil {
		logger.Warn("Could not serialize declaration to JSON", err)
		return
	}

	logger.Debugf("MODULE_DECLARATION: %s", string(bytes))
	logger.Info(eventType)
	if err := b.client.Emit(eventType, string(bytes)); err != nil {
		logger.Warn("Could not send declaration", err)
	}
}

func (b *runner) sendModuleConfigSchema() {
	s := schema.GenerateConfigSchema(b.remoteConfigPtr)
	req := schema.ConfigSchema{Version: b.moduleInfo.ModuleVersion, Schema: s}
	if defaultCfg, err := schema.ExtractConfig(b.defaultRemoteConfigPath); err != nil {
		logger.Error("could not read default config", err)
	} else {
		req.DefaultConfig = defaultCfg
	}
	if bytes, err := json.Marshal(req); err != nil {
		logger.Error("Could not serialize config schema to JSON", err)
	} else if res, err := b.client.Ack(utils.ModuleSendConfigSchema, string(bytes), 3*time.Second); err != nil {
		logger.Error("Could not send config schema", err)
	} else {
		logger.Debugf("Update schema response: %s", res)
	}
}

func (b *runner) sendModuleReady() {
	b.sendModuleDeclaration(utils.ModuleReady)
}

// returns module initial state from bootstrap configuration
func (b *runner) initialState() (remoteConfigReady, requiredModulesReady, routesReady bool, currentConnectedModules map[string]bool) {
	remoteConfigReady = false
	currentConnectedModules = make(map[string]bool)
	for evt, c := range b.requiredModules {
		if !c.mustConnect {
			currentConnectedModules[evt] = true
		}
	}
	requiredModulesReady = len(b.requiredModules) == len(currentConnectedModules)
	routesReady = b.onRoutesReceive == nil
	return
}

func makeRunner(cfg bootstrapConfiguration) *runner {
	return &runner{
		bootstrapConfiguration: cfg,
		remoteConfigChan:       make(chan string),
		connectEventChan:       make(chan connectEvent),
		exitChan:               make(chan struct{}),
		routesChan:             make(chan structure.RoutingConfig),
		disconnectChan:         make(chan struct{}),
	}
}
