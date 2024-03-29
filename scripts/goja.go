package scripts

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

type Script struct {
	prog *goja.Program
}

func NewScript(source ...[]byte) (Script, error) {
	prog, err := goja.Compile("script", string(bytes.Join(source, []byte("\n"))), false)
	return Script{prog: prog}, err
}

type Engine struct {
	pool *sync.Pool
}

func NewEngine() *Engine {
	return &Engine{&sync.Pool{
		New: func() interface{} {
			vm := goja.New()
			return vm
		},
	}}
}

func (m *Engine) Execute(s Script, arg interface{}, opts ...ExecOption) (interface{}, error) {
	vm := m.pool.Get().(*goja.Runtime)

	config := &configOptions{
		arg:           arg,
		scriptTimeout: 2 * time.Second,
	}
	for _, o := range opts {
		o(config)
	}
	var timer *time.Timer
	isAlive := int32(1)
	defer func() {
		atomic.StoreInt32(&isAlive, 0)
		timer.Stop()
		vm.ClearInterrupt()
		m.pool.Put(vm)
	}()

	config.set(vm)
	defer config.unset(vm)

	timer = time.AfterFunc(config.scriptTimeout, func() {
		if atomic.LoadInt32(&isAlive) == 1 {
			vm.Interrupt("execution timeout")
		}
	})
	res, err := vm.RunProgram(s.prog)
	if err != nil {
		return nil, castErr(err)
	}
	return res.Export(), nil
}

func castErr(err error) error {
	if exception, ok := err.(*goja.Exception); ok {
		val := exception.Value().Export()
		if castedErr, ok := val.(error); ok {
			return castedErr
		}
	}
	return err
}
