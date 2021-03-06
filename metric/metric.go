package metric

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/integration-system/isp-lib/v2/structure"
	log "github.com/integration-system/isp-log"
	"github.com/integration-system/isp-log/stdcodes"
	"github.com/rcrowley/go-metrics"
	"github.com/valyala/fasthttp"
)

var (
	statusCheckers = make(map[string]func() interface{}, 0)
	registry       metrics.Registry
	metricServer   *fasthttp.Server
	lock           sync.Mutex
)

const (
	defaultCollectingPeriod = 10
	defaultIpAddress        = "0.0.0.0"
	defaultMetricPath       = "/metrics"
)

func init() {
	registry = metrics.NewRegistry()
	/*_ = metrics.NewRegisteredFunctionalGauge("go routine count", registry, func() int64 {
		return int64(runtime.NumGoroutine())
	})*/
}

func GetRegistry() metrics.Registry {
	return registry
}

func InitHttpServer(metricConfig structure.MetricConfiguration) {
	if metricConfig.Address.Port == "" {
		log.Errorf(stdcodes.ModuleMetricServiceError, "port for metric service must be specified")
		return
	}
	metricPath := metricConfig.Address.Path
	if metricPath == "" {
		metricPath = defaultMetricPath
	}
	if metricConfig.Address.IP == "" {
		metricConfig.Address.IP = defaultIpAddress
	}

	router := fasthttprouter.New()
	router.GET(metricPath, handleMetricRequest)

	router.GET(startProfilingPath, handleEnableProfilingRequest)
	router.GET(stopProfilingPath, handleDisableProfilingRequest)

	router.GET("/swagger/*name", makeSwaggerHandler(metricConfig.Address.AddressConfiguration))

	lock.Lock()
	newMetricServer := &fasthttp.Server{
		Handler:      router.Handler,
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
	}

	if metricServer != nil {
		_ = metricServer.Shutdown()
	}
	metricServer = newMetricServer
	lock.Unlock()

	listenAddr := metricConfig.Address.GetAddress()
	log.Infof(0, "start metric service on %s", listenAddr)
	go func() {
		err := newMetricServer.ListenAndServe(listenAddr)
		if err != nil {
			log.Errorf(stdcodes.ModuleMetricServiceError, "could not start metric service: %v", err)
		}
	}()
}

func InitCollectors(newMetricConfig structure.MetricConfiguration,
	oldMetricConfig structure.MetricConfiguration) metrics.Registry {

	if newMetricConfig.Gc != oldMetricConfig.Gc ||
		newMetricConfig.Memory != oldMetricConfig.Memory {
		registry.UnregisterAll()
		if newMetricConfig.Gc {
			collectingGCPeriod := newMetricConfig.CollectingGCPeriod
			if collectingGCPeriod == 0 {
				collectingGCPeriod = defaultCollectingPeriod
			}
			InitGCMetrics(time.Duration(collectingGCPeriod) * time.Second)
		}
		if newMetricConfig.Memory {
			collectingMemoryPeriod := newMetricConfig.CollectingMemoryPeriod
			if collectingMemoryPeriod == 0 {
				collectingMemoryPeriod = defaultCollectingPeriod
			}
			InitMemoryMetrics(time.Duration(collectingMemoryPeriod) * time.Second)
		}
	}
	return registry
}

func InitGCMetrics(duration time.Duration) {
	metrics.RegisterDebugGCStats(registry)
	go metrics.CaptureDebugGCStats(registry, duration)
}

func InitMemoryMetrics(duration time.Duration) {
	metrics.RegisterRuntimeMemStats(registry)
	go metrics.CaptureRuntimeMemStats(registry, duration)
}

func InitHealhcheck(name string, checker func(h metrics.Healthcheck)) {
	hc := metrics.NewHealthcheck(checker)
	_ = registry.Register(name, hc)
}

func InitStatusChecker(name string, checker func() interface{}) {
	statusCheckers[name] = checker
}

func RemoveStatusChecker(name string) {
	delete(statusCheckers, name)
}

func RemoveAllStatusChecker() {
	statusCheckers = make(map[string]func() interface{}, 0)
}

func handleMetricRequest(ctx *fasthttp.RequestCtx) {
	registry.RunHealthchecks()
	allMetrics := registry.GetAll()
	if len(statusCheckers) != 0 {
		statuses := map[string]interface{}{}
		for k, v := range statusCheckers {
			statuses[k] = v()
		}
		allMetrics["status"] = statuses
	}
	bytes, _ := json.Marshal(allMetrics)
	ctx.SetContentType("application/json")
	ctx.SetBody(bytes)
	ctx.SetStatusCode(fasthttp.StatusOK)
}
