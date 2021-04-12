### v2.8.5
* bootstrap: fix error wrapping from govalidator when schema of remote config has required map field
### v2.8.4
* moved event entities to event-lib
* fix jsonschema cross package definition collision
### v2.8.3
* bootstrap: fix empty connection string
* bootstrap: always send module requirements
### v2.8.2
* bootstrap: fix outbound ip detection
* bootstrap: remove unnecessary double close error log 
### v2.8.1
* fix release tag
### v2.8.0
* bootstrap: remove unnecessary subscription to isp-gate
* csvwriter: added new option (`WithGzipCompression`)
* streaming: added new writer for streaming
* added package `scripts` with goja wrapping
### v2.7.0
* bootstrap: add handling errors from `ackEvent` which resolved deadlock of **main select**
* bootstrap: add tests that check protocol interaction and test framework for it
* update protobuf and grpc
* fix panics when service has no swagger doc
### v2.6.2
* add functionality to send requests from swagger-ui
### v2.6.1
* new `docs` package with swagger doc
### v2.6.0
* add swagger-ui on metrics port
* upgrade deps
### v2.5.0
* add full module requirements to declaration
* add restart event listener
* add redis sentinel support
* log override config
### v2.4.0
* bootstrap: add function to subscribe to broadcasting events from config service
### v2.3.1
* set jsoniter as default go-pg json provider
* fix deadlock in metric InitHttpServer
* fix bug that previous metricPath was still available
* fix panic when re-init metric server
* add elapsed_time to db log query hook
### v2.3.0
* rewrite grpc client
* add profiling endpoints to metric port
* remove deprecated/unused packages: grpc-proxy, logger, modules
* add heartbeat to bootstrap package
### v2.2.0
* **Warning: this and further releases are incompatible with golang/dep**
### v2.0.1
* fix deadlock in initChan
* fix bytes logging
* remove unnecessary ws disconnection error on shutdown
* increase default websocket ConnectionReadLimit to 4 MB, add to configuration
### v2.0.0
* (*) migrate to `isp-etp` websocket transport and `isp-config-service` 2.0
### v1.9.0
* (*) full reformat packages, remove useless functions
* (*) new `isp-log` logging
* (+) new `RxRedisClient`
* (-) remove old database client
### v1.6.3
* (*) migrate to new nats.go
* (+) socket.io client now support connecting to many servers
* (+) unmarshaling to dynamic struct in grpc client
### v1.6.2
* (+) add custom generators for json schema
### v1.6.1
* (*) fix schema validators for `enum` and `maximum`
* (*) add `ru` localization for schema description
* (*) fix dereference schemas
### v1.6.0
* (+) add util method for dereference schemas definitions
* (+) add options for http client
* (+) add const for log rotation
* (*) fix csv closing
### v1.5.2
* (*) fix http client in high concurrency env
### v1.5.1
* (+) add csv utils
### v1.5.0
* (+) unsafe methods for set configuration (usefull for tests)
* (*) fix remote configuration receiving
* (*) fix error logging in stream handler
### v1.4.0
* (+) new interceptors and post processors request API for grpc service
* (+) validation API for grpc and http services
* (*) fix panic on metric catching
### v1.3.2
* fix file streaming
### v1.3.1
* (*) use `WriterCloser` instead `os.File`
### v1.3.0
* (-) rabbit client
* (+) code for invoke streaming method in `GrpcClient`
* (+) `Unsafe()` to get db object
* (*) refactoring
### v1.2.2
* fix 'bad request' error
* reformat
### v1.2.1
* reformat backend package
* add sync request grpc interceptor
* add default config path resolver
### v1.2.0
* send default remote config with schema
### v1.1.5
* fix soap content type
### v1.1.4
* add http validation
### v1.1.3
* fix log time
### v1.1.2
* add low level grpc client accessor
### v1.1.1
add `mdm-search/record/count` method
### v1.1.0
* new nats client
* remove lumberjack log rorator
### v1.0.1
* update `github.com/valyala/fasthttp`
### v1.0.0
* full package refactoring
* **compilation incompatible with previous version**
### v0.7.1
* initial
