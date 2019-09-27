package config

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"os"
	"os/signal"
	"path"
	"reflect"
	"strings"
	"sync"
	"syscall"

	"github.com/asaskevich/govalidator"
	"github.com/fsnotify/fsnotify"
	"github.com/integration-system/bellows"
	"github.com/integration-system/isp-lib/utils"
	"github.com/mohae/deepcopy"
	"github.com/spf13/viper"
)

const (
	LocalConfigEnvPrefix  = "LC_ISP"
	RemoteConfigEnvPrefix = "RC_ISP"
)

var (
	configInstance       interface{}
	remoteConfigInstance interface{}

	startWatching  = sync.Once{}
	onChangeFunc   interface{}
	errInvalidFunc = errors.New("expecting func with two pointers to local config type")

	reloadSig = syscall.SIGHUP
)

func init() {
	ex, _ := os.Executable()

	viper.SetEnvPrefix(LocalConfigEnvPrefix)
	viper.AutomaticEnv()

	envConfigName := "config"
	configPath := path.Dir(ex)
	if utils.DEV {
		// _, filename, _, _ := runtime.Caller(0)
		configPath = "./conf/"
	}
	if utils.EnvProfileName != "" {
		envConfigName = "config_" + utils.EnvProfileName + ".yml"
	}
	if utils.EnvConfigPath != "" {
		configPath = utils.EnvConfigPath
	}

	viper.SetConfigName(envConfigName)
	viper.AddConfigPath(configPath)
}

func Get() interface{} {
	return configInstance
}

func GetRemote() interface{} {
	return remoteConfigInstance
}

func UnsafeSetRemote(remoteConfig interface{}) {
	remoteConfigInstance = remoteConfig
}

func UnsafeSet(localConfig interface{}) {
	configInstance = localConfig
}

func InitConfig(configuration interface{}) interface{} {
	return InitConfigV2(configuration, false)
}

func InitConfigV2(configuration interface{}, callOnChangeHandler bool) interface{} {
	if localConfig, err := readLocalConfig(configuration); err != nil {
		log.Fatalf(stdcodes.ModuleReadLocalConfigError, "could not read local config: %v", err)
		return nil
	} else if err := validateConfig(localConfig); err != nil {
		log.Fatalf(stdcodes.ModuleInvalidLocalConfig, "invalid local config: %v", err)
		return nil
	} else {
		configInstance = localConfig
		if callOnChangeHandler {
			handleConfigChange(localConfig, nil)
		}
	}
	return configInstance
}

func InitRemoteConfig(configuration interface{}, remoteConfig string) interface{} {
	newRemoteConfig, err := overrideConfigurationFromEnv(remoteConfig, RemoteConfigEnvPrefix)
	if err != nil {
		log.WithMetadata(log.Metadata{"config": remoteConfig}).
			Fatalf(stdcodes.ModuleOverrideRemoteConfigError, "could not override remote config via env: %v", err)
		return nil
	}

	newConfiguration := reflect.New(reflect.TypeOf(configuration).Elem()).Interface()
	if err := json.Unmarshal([]byte(newRemoteConfig), newConfiguration); err != nil {
		log.WithMetadata(log.Metadata{"data": remoteConfig}).
			Fatalf(stdcodes.ConfigServiceInvalidDataReceived, "received invalid remote config: %v", err)
	} else if err := validateConfig(newConfiguration); err != nil {
		log.WithMetadata(log.Metadata{"config": remoteConfig}).
			Fatalf(stdcodes.ModuleInvalidRemoteConfig, "received invalid remote config: %v", err)
	} else {
		remoteConfigInstance = newConfiguration
	}

	return remoteConfigInstance
}

// Example:
// config.OnConfigChange(func(new, old *conf.Configuration) {
//
// })
// Callback call after initial loading and after every config files changing.
// On first call new and old configurations are equals
func OnConfigChange(f interface{}) {
	rt := reflect.TypeOf(f)
	if rt.Kind() != reflect.Func || rt.NumIn() != 2 {
		panic(errInvalidFunc)
		return
	}

	onChangeFunc = f

	startWatching.Do(func() {
		viper.WatchConfig()
		viper.OnConfigChange(func(in fsnotify.Event) {
			reloadConfig()
		})
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, reloadSig)
		go func() {
			for {
				_, ok := <-sigChan
				if !ok {
					return
				}
				reloadConfig()
			}
		}()
	})
}

func reloadConfig() {
	old := deepcopy.Copy(configInstance)
	newConfig, err := readLocalConfig(configInstance)
	if err != nil {
		log.Errorf(stdcodes.ModuleReadLocalConfigError, "could not read local config: %v", err)
		return
	}
	if err := validateConfig(newConfig); err != nil {
		log.Errorf(stdcodes.ModuleInvalidLocalConfig, "invalid local config: %v", err)
		configInstance = old
	} else {
		configInstance = newConfig
		handleConfigChange(newConfig, old)
	}
}

func readLocalConfig(config interface{}) (interface{}, error) {
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read local config file: %v", err)
	} else if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %v", err)
	}
	return config, nil
}

func handleConfigChange(newConfig, oldConfig interface{}) {
	if onChangeFunc == nil {
		return
	}

	rv := reflect.ValueOf(onChangeFunc)
	rt := rv.Type()
	configType := reflect.TypeOf(newConfig).String()
	newCfgType := rt.In(0).String()
	oldCfgType := rt.In(1).String()
	if newCfgType == oldCfgType && newCfgType == configType {
		if oldConfig == nil {
			oldConfig = newConfig
		}
		args := []reflect.Value{reflect.ValueOf(newConfig), reflect.ValueOf(oldConfig)}
		rv.Call(args)
	} else {
		panic(errInvalidFunc)
	}
}

func validateConfig(cfg interface{}) error {
	if _, err := govalidator.ValidateStruct(cfg); err != nil {
		validationErrors := govalidator.ErrorsByField(err)
		str := strings.Builder{}
		for k, v := range validationErrors {
			str.WriteString(k)
			str.WriteString(" -> ")
			str.WriteString(v)
			str.WriteString(", ")
		}
		return errors.New(str.String())
	} else {
		return nil
	}
}

func overrideConfigurationFromEnv(src string, envPrefix string) (string, error) {
	envPrefix = envPrefix + "_"
	overrides := getEnvOverrides(envPrefix)
	if len(overrides) == 0 {
		return src, nil
	}

	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(src), &m)
	if err != nil {
		return "", fmt.Errorf("unmarshal to map: %v", err)
	}

	m = bellows.Flatten(m)
	flattenMap := make(map[string]interface{}, len(m))
	for k, v := range m {
		flattenMap[strings.ToLower(k)] = v
	}

	for path, val := range overrides {
		if newValue, err := castString(val); err != nil {
			return "", fmt.Errorf("could not override remote config variable %s, new value: %v, err: %v", path, val, err)
		} else {
			flattenMap[path] = newValue
		}
	}

	expandedMap := bellows.Expand(flattenMap)
	bytes, err := json.Marshal(expandedMap)
	if err != nil {
		return "", fmt.Errorf("marhal to json: %v", err)
	}

	return string(bytes), nil
}
