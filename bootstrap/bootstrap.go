package bootstrap

import (
	"errors"
	"reflect"

	"github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"github.com/sirupsen/logrus"
	"github.com/thecodeteam/goodbye"
)

const (
	LibraryVersion = "2.6.2"
)

func init() {
	if utils.DEV {
		must(log.SetLevel(logrus.DebugLevel.String()))
	}
}

type bootstrapConfiguration struct {
	localConfigPtr   interface{}
	localConfigType  string
	remoteConfigPtr  interface{}
	remoteConfigType string

	defaultRemoteConfigPath string

	onLocalConfigLoad     *reflect.Value
	onRemoteConfigReceive *reflect.Value
	onSocketErrorReceive  *reflect.Value
	onConfigErrorReceive  *reflect.Value
	onRoutesReceive       routesConsumer
	onLocalConfigChange   interface{}
	onShutdown            shutdownHandler
	onModuleReady         func()

	// module name -> consumer
	requiredModules map[string]*connectConsumer
	// module name -> addresses
	connectedModules map[string][]string
	subscribedEvents map[string]func([]byte)

	makeSocketConfig   socketConfigProducer
	makeModuleInfo     moduleInfoProducer
	declaratorAcquirer declaratorAcquirer
}

/**
 * Add an event listener for the moment when the local config for an application loaded
 */
func (cfg *bootstrapConfiguration) OnLocalConfigLoad(f interface{}) *bootstrapConfiguration {
	rv, rt := reflect.ValueOf(f), reflect.TypeOf(f)
	assertSingleParamFunc(rt, cfg.localConfigType)
	cfg.onLocalConfigLoad = &rv
	return cfg
}

/**
 * Add an event listener for the moment when the local config for current application changed
 */
func (cfg *bootstrapConfiguration) OnLocalConfigChange(f interface{}) *bootstrapConfiguration {
	rt := reflect.TypeOf(f)
	assertTwoParamFunc(rt, cfg.localConfigType)
	cfg.onLocalConfigChange = f
	return cfg
}

/**
 * Add an event listener for the moment when the error message receive
 */
func (cfg *bootstrapConfiguration) OnSocketErrorReceive(f interface{}) *bootstrapConfiguration {
	rv, rt := reflect.ValueOf(f), reflect.TypeOf(f)
	assertSingleParamFunc(rt, reflect.TypeOf(map[string]interface{}{}).String())
	cfg.onSocketErrorReceive = &rv
	return cfg
}

/**
 * Add an event listener for the moment when the config error message receive
 */
func (cfg *bootstrapConfiguration) OnConfigErrorReceive(f interface{}) *bootstrapConfiguration {
	rv, rt := reflect.ValueOf(f), reflect.TypeOf(f)
	assertSingleParamFunc(rt, "string")
	cfg.onConfigErrorReceive = &rv
	return cfg
}

/**
 * Add an event listener for the moment when an application received its configuration
 */
func (cfg *bootstrapConfiguration) OnRemoteConfigReceive(f interface{}) *bootstrapConfiguration {
	if cfg.remoteConfigType == "" {
		panic(errors.New("remote config type is undefined"))
		return nil
	}
	rv, rt := reflect.ValueOf(f), reflect.TypeOf(f)
	assertTwoParamFunc(rt, cfg.remoteConfigType)
	cfg.onRemoteConfigReceive = &rv
	return cfg
}

/**
 * Add a hook for executing a code when an application is ready to be ended
 */
func (cfg *bootstrapConfiguration) OnShutdown(f shutdownHandler) *bootstrapConfiguration {
	cfg.onShutdown = f
	return cfg
}

/**
 * Specify the socket builder function that creates a socket configuration
 */
func (cfg *bootstrapConfiguration) SocketConfiguration(f socketConfigProducer) *bootstrapConfiguration {
	cfg.makeSocketConfig = f
	return cfg
}

// set callback function which receive module declarator on startup
func (cfg *bootstrapConfiguration) AcquireDeclarator(f declaratorAcquirer) *bootstrapConfiguration {
	cfg.declaratorAcquirer = f
	return cfg
}

// provides callback function which return base module information
func (cfg *bootstrapConfiguration) DeclareMe(f moduleInfoProducer) *bootstrapConfiguration {
	cfg.makeModuleInfo = f
	return cfg
}

// module is in not ready state until received routes from config-service
func (cfg *bootstrapConfiguration) RequireRoutes(f routesConsumer) *bootstrapConfiguration {
	cfg.onRoutesReceive = f
	return cfg
}

// module is in not ready state until establish grpc connection with required modules
func (cfg *bootstrapConfiguration) RequireModule(moduleName string, consumer addressListConsumer, mustConnect bool) *bootstrapConfiguration {
	cfg.requiredModules[moduleName] = &connectConsumer{mustConnect: mustConnect, consumer: consumer}
	return cfg
}

// add path to remote config module
func (cfg *bootstrapConfiguration) DefaultRemoteConfigPath(path string) *bootstrapConfiguration {
	cfg.defaultRemoteConfigPath = path
	return cfg
}

// subscribe to event published from config service
// note: data slice is reused, so for async handling or storing, data must be copied
func (cfg *bootstrapConfiguration) SubscribeBroadcastEvent(event string, f func(data []byte)) *bootstrapConfiguration {
	cfg.subscribedEvents[event] = f
	return cfg
}

// callback fires every time before MODULE:READY send to config service
func (cfg *bootstrapConfiguration) OnModuleReady(f func()) *bootstrapConfiguration {
	cfg.onModuleReady = f
	return cfg
}

// starts module, block until interruption
func (cfg *bootstrapConfiguration) Run() {
	runner := makeRunner(*cfg)
	defer func() {
		goodbye.Exit(runner.ctx, 0)
	}()
	err := runner.run()
	if err != nil {
		//level `logrus.FatalLevel` will not lead to os.Exit()
		log.Logf(logrus.FatalLevel, stdcodes.ModuleRunFatalError, "could not run module, fatal error occurred: %+v", err)
	}
}

// entry point to describe module
func ServiceBootstrap(localConfigPtr, remoteConfigPtr interface{}) *bootstrapConfiguration {
	if localConfigPtr == nil || reflect.TypeOf(localConfigPtr).Kind() != reflect.Ptr {
		panic(errors.New("expecting not nil pointer to local config struct"))
		return nil
	}
	if remoteConfigPtr != nil && reflect.TypeOf(remoteConfigPtr).Kind() != reflect.Ptr {
		panic(errors.New("expecting not nil pointer to remote config struct"))
		return nil
	}
	b := &bootstrapConfiguration{
		localConfigPtr:   localConfigPtr,
		localConfigType:  reflect.TypeOf(localConfigPtr).String(),
		requiredModules:  make(map[string]*connectConsumer),
		connectedModules: make(map[string][]string),
		subscribedEvents: make(map[string]func([]byte)),
	}
	if remoteConfigPtr != nil {
		b.remoteConfigPtr = remoteConfigPtr
		b.remoteConfigType = reflect.TypeOf(remoteConfigPtr).String()
	}

	return b
}
