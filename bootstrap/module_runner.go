package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	etp "github.com/integration-system/isp-etp-go/client"
	"github.com/integration-system/isp-lib/v2/config"
	"github.com/integration-system/isp-lib/v2/config/schema"
	"github.com/integration-system/isp-lib/v2/metric"
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"github.com/mohae/deepcopy"
	"github.com/thecodeteam/goodbye"
	"nhooyr.io/websocket"
)

const (
	defaultConfigServiceConnectionTimeout       = 400 * time.Millisecond
	defaultRemoteConfigAwaitTimeout             = 3 * time.Second
	defaultMaxAckRetryTimeout                   = 10 * time.Second
	defaultConnectionReadLimit            int64 = 4 << 20 // 4 MB
)

type runner struct {
	bootstrapConfiguration

	moduleInfo ModuleInfo

	remoteConfigChan chan []byte
	routesChan       chan structure.RoutingConfig
	connectEventChan chan connectEvent
	disconnectChan   chan struct{}

	client                   etp.Client
	connStrings              *RoundRobinStrings
	ready                    bool
	lastFailedConnectionTime time.Time

	ctx context.Context
}

func (b *runner) run() {
	ctx := b.initShutdownHandler()
	b.ctx = ctx
	defer goodbye.Exit(ctx, 0)

	defer func() {
		err := recover()
		if err != nil {
			debug.PrintStack()
			log.Fatalf(stdcodes.ModuleRunFatalError, "could not run module, fatal error occurred: %v", err)
		}
	}()

	b.initLocalConfig()                //read local configuration, calls callback
	b.initModuleInfo()                 //set moduleInfo
	client := b.initSocketConnection() //create socket object, subscribe to all events
	if client == nil {
		return
	}
	b.client = client
	b.initStatusMetrics() //add socket and required modules connections checkers in metrics

	if b.declaratorAcquirer != nil {
		b.declaratorAcquirer(&declarator{b.sendModuleDeclaration}) //provides module declarator to clients code
	}

	go b.sendModuleConfigSchema() //create and send schema with default remote config

	b.ready = false //module not ready state by default

	remoteConfigReady, requiredModulesReady, routesReady, currentConnectedModules := b.initialState()
	remoteConfigTimeoutChan := time.After(defaultRemoteConfigAwaitTimeout) //used for log WARN message
	neverTriggerChan := make(chan time.Time)                               //used for stops log flood
	initChan := make(chan struct{}, 1)
	//in main goroutine handle all asynchronous events from config service
	for {
		//if all conditions are true, put signal into channel and later in loop send MODULE:READY event to config-service
		if !b.ready && remoteConfigReady && requiredModulesReady && routesReady {
			b.ready = true
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
				go b.sendModuleRequirements() //after first time receiving config, send requirements
			}

			remoteConfigTimeoutChan = neverTriggerChan //stop flooding in logs
		case <-remoteConfigTimeoutChan:
			log.Error(stdcodes.RemoteConfigIsNotReceivedByTimeout, "remote config is not received by timeout")
			remoteConfigTimeoutChan = time.After(defaultRemoteConfigAwaitTimeout)
		case routers := <-b.routesChan:
			if b.onRoutesReceive != nil {
				routesReady = b.onRoutesReceive(routers)
			}
		case e := <-b.connectEventChan:
			if c, ok := b.requiredModules[e.module]; ok {
				if ok := c.consumer(e.addressList); ok {
					currentConnectedModules[e.module] = true
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
				if currentConnectedModules[e.module] {
					for _, addr := range e.addressList {
						addrList = append(addrList, addr.GetAddress())
					}
				}
				b.connectedModules[e.module] = addrList
			}
		case <-initChan:
			if b.onModuleReady != nil {
				b.onModuleReady()
			}
			go b.sendModuleReady()
		case <-b.disconnectChan: //on disconnection, set state to 'not ready' once again
			b.ready = false
			remoteConfigReady, requiredModulesReady, routesReady, currentConnectedModules = b.initialState()
			select {
			case <-b.ctx.Done():
				return
			case <-time.After(defaultConfigServiceConnectionTimeout):
			}
			client := b.initSocketConnection()
			// true only if exitChan closed
			if client == nil {
				return
			}
			b.client = client
			go b.sendModuleConfigSchema()
		case <-b.ctx.Done(): //return from main goroutine after shutdown signal
			return
		}

	}
}

func (b *runner) initShutdownHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	goodbye.Notify(ctx)
	goodbye.Register(func(ctx context.Context, sig os.Signal) {
		log.Info(stdcodes.ModuleManualShutdown, "module shutting down now")

		cancel()
		if b.client != nil && !b.client.Closed() {
			_ = b.client.Close()
		}

		if b.onShutdown != nil {
			b.onShutdown(ctx, sig)
		}

		log.Info(stdcodes.ModuleManualShutdown, "module has gracefully shut down")
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

func (b *runner) initSocketConnection() etp.Client {
	if b.makeSocketConfig == nil {
		panic(errors.New("socket configuration is not specified. Call 'SocketConfiguration' first"))
	}

	socketConfig := b.makeSocketConfig(b.localConfigPtr)
	if b.connStrings == nil {
		connectionStrings, err := getConfigServiceConnectionStrings(socketConfig)
		if err != nil {
			panic(err)
		}
		b.connStrings = NewRoundRobinStrings(connectionStrings)
	}

	connectionReadLimit := defaultConnectionReadLimit
	if socketConfig.ConnectionReadLimitKB > 0 {
		connectionReadLimit = socketConfig.ConnectionReadLimitKB << 10
	}
	etpConfig := etp.Config{
		HttpClient:          http.DefaultClient,
		ConnectionReadLimit: connectionReadLimit,
	}
	client := etp.NewClient(etpConfig)
	client.OnDisconnect(func(err error) {
		if websocket.CloseStatus(err) != websocket.StatusNormalClosure && !errors.Is(err, context.Canceled) {
			log.Errorf(stdcodes.ConfigServiceDisconnection, "disconnected from config service: %v", err)
		}
		b.lastFailedConnectionTime = time.Now()
		b.disconnectChan <- struct{}{}
	})

	if b.onSocketErrorReceive != nil {
		client.On(utils.ErrorConnection, handleError(b.onSocketErrorReceive, utils.ErrorConnection))
	}
	if b.onConfigErrorReceive != nil {
		client.On(utils.ConfigError, handleConfigError(b.onConfigErrorReceive, utils.ConfigError))
	}
	if b.remoteConfigPtr != nil {
		client.On(utils.ConfigSendConfigWhenConnected, handleRemoteConfiguration(b.remoteConfigChan, utils.ConfigSendConfigWhenConnected))
		client.On(utils.ConfigSendConfigChanged, handleRemoteConfiguration(b.remoteConfigChan, utils.ConfigSendConfigChanged))
		client.On(utils.ConfigSendConfigOnRequest, handleRemoteConfiguration(b.remoteConfigChan, utils.ConfigSendConfigOnRequest))
	}
	if b.onRoutesReceive != nil {
		client.On(utils.ConfigSendRoutesChanged, handleRoutes(b.routesChan, utils.ConfigSendRoutesChanged))
		client.On(utils.ConfigSendRoutesWhenConnected, handleRoutes(b.routesChan, utils.ConfigSendRoutesWhenConnected))
		client.On(utils.ConfigSendRoutesOnRequest, handleRoutes(b.routesChan, utils.ConfigSendRoutesOnRequest))
	}
	for module := range b.requiredModules {
		event := utils.ModuleConnected(module)
		client.On(event, UnmarshalAddressListAndThen(event, makeAddressListConsumer(module, b.connectEventChan)))
	}

	err := client.Dial(b.ctx, b.connStrings.Get())
	for err != nil {
		log.Errorf(stdcodes.ConfigServiceConnectionError, "could not connect to config service: %v", err)
		b.lastFailedConnectionTime = time.Now()

		select {
		case <-b.ctx.Done():
			return nil
		case <-time.After(defaultConfigServiceConnectionTimeout):

		}
		err = client.Dial(b.ctx, b.connStrings.Get())
	}

	return client
}

func (b *runner) initStatusMetrics() {
	metric.InitStatusChecker("config-websocket", func() interface{} {
		socketConfig := b.makeSocketConfig(b.localConfigPtr)
		uri := fmt.Sprintf("%s:%s", socketConfig.Host, socketConfig.Port)
		status := true
		if b.client == nil || b.client.Closed() {
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

	for module := range b.requiredModules {
		moduleCopy := module
		metric.InitStatusChecker(fmt.Sprintf("%s-grpc", module), func() interface{} {
			addrList, ok := b.connectedModules[moduleCopy]
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
		bf := getDefaultBackoff(b.ctx, defaultMaxAckRetryTimeout)
		if ok, bytes, res := ackEvent(b.client, utils.ModuleSendRequirements, requirements, bf); ok {
			log.WithMetadata(log.Metadata{"event": utils.ModuleSendRequirements, "data": string(bytes), "response": string(res)}).
				Info(stdcodes.ConfigServiceSendRequirements, "send module requirements")
		}
	}
}

func (b *runner) sendModuleDeclaration(eventType string) {
	b.moduleInfo = b.makeModuleInfo(b.localConfigPtr)

	declaration := getModuleDeclaration(b.moduleInfo)

	bf := getDefaultBackoff(b.ctx, defaultMaxAckRetryTimeout)
	if ok, bytes, res := ackEvent(b.client, eventType, declaration, bf); ok {
		log.WithMetadata(log.Metadata{"event": eventType, "data": string(bytes), "response": string(res)}).
			Info(stdcodes.ConfigServiceSendModuleReady, "send module declaration")
	}
}

func (b *runner) sendModuleConfigSchema() {
	s := schema.GenerateConfigSchema(b.remoteConfigPtr)
	req := schema.ConfigSchema{Version: b.moduleInfo.ModuleVersion, Schema: s}

	if defaultCfg, err := schema.ExtractConfig(b.defaultRemoteConfigPath); err != nil {
		log.WithMetadata(log.Metadata{"path": b.defaultRemoteConfigPath}).
			Warnf(stdcodes.ModuleDefaultRCReadError, "could not read default remote config: %v", err)
	} else {
		req.DefaultConfig = defaultCfg
	}

	bf := getDefaultBackoff(b.ctx, defaultMaxAckRetryTimeout)
	if ok, _, resp := ackEvent(b.client, utils.ModuleSendConfigSchema, req, bf); ok {
		log.WithMetadata(log.Metadata{"response": string(resp)}).
			Info(stdcodes.ConfigServiceSendConfigSchema, "send config schema and default config")
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
		remoteConfigChan:       make(chan []byte),
		connectEventChan:       make(chan connectEvent),
		routesChan:             make(chan structure.RoutingConfig),
		disconnectChan:         make(chan struct{}),
	}
}
