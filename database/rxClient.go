package database

import (
	"errors"
	"fmt"
	"github.com/go-pg/pg"
	"github.com/integration-system/go-cmp/cmp"
	"sync"
)

var (
	ErrNotConnected = errors.New("db: not connected")
)

type ErrorEvent struct {
	action string
	err    error
	config DBConfiguration
}

func (er ErrorEvent) Error() string {
	return fmt.Sprintf("rxDbClient: %s: %v, config: %v", er.action, er.err, er.config)
}

type errorHandler func(err *ErrorEvent)

type visitor func(db *pg.DB) error

type initHandler func(db *pg.DB, config DBConfiguration)

type RxDbClient struct {
	db       *pg.DB
	lastConf DBConfiguration
	lock     sync.RWMutex
	active   bool

	initHandler     initHandler
	eh              errorHandler
	ensMigrations   bool
	ensSchema       bool
	schemaInjecting bool
}

func (rc *RxDbClient) ReceiveConfiguration(config DBConfiguration) {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	if !rc.active {
		return
	}

	if !cmp.Equal(rc.lastConf, config) {
		ok := true

		if rc.ensSchema && config.CreateSchema {
			if err := ensureSchemaExists(config); err != nil {
				if rc.eh != nil {
					rc.eh(&ErrorEvent{"check schema", err, config})
				}
				ok = false
			}
		}

		if ok && rc.ensMigrations {
			if err := ensureMigrations(config); err != nil {
				if rc.eh != nil {
					rc.eh(&ErrorEvent{"run migrations", err, config})
				}
				ok = false
			}
		}

		var db *pg.DB
		if ok {
			if pdb, err := NewDbConnection(config); err != nil {
				if rc.eh != nil {
					rc.eh(&ErrorEvent{"connect", err, config})
				}
				ok = false
			} else {
				db = pdb
			}

			if ok && rc.schemaInjecting {
				db = withSchema(db, config.Schema)
			}
		}

		if ok && rc.db != nil {
			rc.db.Close()
			rc.db = nil
		}

		if ok {
			rc.db = db
			rc.lastConf = config
			if rc.initHandler != nil {
				rc.initHandler(rc.db, config)
			}
		}
	}
}

func (rc *RxDbClient) Close() error {
	rc.lock.Lock()
	defer rc.lock.Unlock()

	rc.active = false
	if rc.db != nil {
		db := rc.db
		rc.db = nil
		return db.Close()
	}
	return nil
}

func (rc *RxDbClient) Visit(v visitor) error {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	if rc.db == nil {
		return ErrNotConnected
	}

	return v(rc.db)
}

func (rc *RxDbClient) RunInTransaction(f interface{}) error {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	if rc.db == nil {
		return ErrNotConnected
	}

	return RunInTransactionV2(rc.db, f)
}

func NewRxDbClient(opts ...Option) *RxDbClient {
	rdc := &RxDbClient{active: true}

	for _, o := range opts {
		o(rdc)
	}

	return rdc
}
