package bootstrap

import (
	"github.com/integration-system/isp-lib/v2/structure"
	"github.com/integration-system/isp-lib/v2/utils"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"github.com/json-iterator/go"
	"os"
	"reflect"
	"syscall"
)

const (
	configServiceEvent = 87
)

var (
	json = jsoniter.ConfigFastest
)

func UnmarshalAddressListAndThen(event string, f func([]structure.AddressConfiguration)) func([]byte) {
	return func(data []byte) {
		list := make([]structure.AddressConfiguration, 0)
		if err := json.Unmarshal(data, &list); err != nil {
			log.WithMetadata(log.Metadata{"event": event, "data": string(data)}).
				Errorf(stdcodes.ConfigServiceInvalidDataReceived, "received invalid address list: %v", err)
		} else {
			log.WithMetadata(log.Metadata{"event": event, "data": string(data)}).
				Info(stdcodes.ConfigServiceReceiveRequiredModuleAddress, "received required module address list")
			f(list)
		}
	}
}

// kill proc itself, systemd will up service again
func ListenRestartEvent() (string, func(data []byte)) {
	return utils.ConfigRestart, func(_ []byte) {
		log.WithMetadata(log.Metadata{"event": utils.ConfigRestart}).Info(configServiceEvent, "kill itself")
		p, err := os.FindProcess(syscall.Getpid())
		if err != nil {
			log.WithMetadata(log.Metadata{"event": utils.ConfigRestart}).
				Errorf(configServiceEvent, "find proc: %v", err)
			return
		}
		if err := p.Signal(syscall.SIGTERM); err != nil {
			log.WithMetadata(log.Metadata{"event": utils.ConfigRestart}).
				Errorf(configServiceEvent, "kill: %v", err)
		}
	}
}

func handleRemoteConfiguration(remoteConfigChan chan []byte, event string) func([]byte) {
	return func(data []byte) {
		copyedSl := make([]byte, len(data))
		copy(copyedSl, data)
		remoteConfigChan <- copyedSl
	}
}

func handleError(onSocketErrorReceive *reflect.Value, event string) func([]byte) {
	return func(data []byte) {
		var args map[string]interface{}
		_ = json.Unmarshal(data, &args)
		callFunc(onSocketErrorReceive, args)
	}
}

func handleConfigError(onConfigErrorReceive *reflect.Value, event string) func([]byte) {
	return func(data []byte) {
		callFunc(onConfigErrorReceive, string(data))
	}
}

func handleRoutes(routesChan chan structure.RoutingConfig, event string) func([]byte) {
	return func(data []byte) {
		routes := structure.RoutingConfig{}
		err := json.Unmarshal(data, &routes)
		if err != nil {
			log.WithMetadata(log.Metadata{"event": event, "data": string(data)}).
				Errorf(stdcodes.ConfigServiceInvalidDataReceived, "received invalid routes list: %v", err)
			return
		}

		if err := utils.Validate(routes); err == nil {
			totalModules := len(routes)
			totalEndpoints := 0
			for _, v := range routes {
				totalEndpoints += len(v.Endpoints)
			}
			log.WithMetadata(log.Metadata{"total_modules": totalModules, "total_endpoints": totalEndpoints}).
				Info(stdcodes.ConfigServiceReceiveRoutes, "received routes")
			routesChan <- routes
		} else {
			log.WithMetadata(log.Metadata{"event": event, "data": string(data)}).
				Errorf(stdcodes.ConfigServiceInvalidDataReceived, "received invalid routes list: %v", err)
		}
	}
}

func (b *runner) handleArbitraryEvent(event string, data []byte) {
	f := b.subscribedEvents[event]
	if f != nil {
		f(data)
	}
}
