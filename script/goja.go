package script

import (
	"bytes"
	"sync"
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

type Machine struct {
	pool   *sync.Pool
	config configOptions
}

func NewMachine(opts ...ExecOption) *Machine {
	m := &Machine{}
	m.config = configOptions{}
	for _, o := range opts {
		o(&m.config)
	}
	m.pool = &sync.Pool{
		New: func() interface{} {
			vm := goja.New()
			m.config.set(vm)
			return vm
		},
	}
	return m
}

func (m *Machine) Execute(s Script, arg interface{}, opts ...ExecOption) (interface{}, error) {
	vm := m.pool.Get().(*goja.Runtime)
	defer m.config.unset(vm)

	config := &configOptions{
		arg:           arg,
		scriptTimeout: 2 * time.Second,
	}
	for _, o := range opts {
		o(config)
	}
	config.timer = time.AfterFunc(config.scriptTimeout, func() {
		vm.Interrupt("execution timeout")
	})
	defer func() {
		config.timer.Stop()
		vm.ClearInterrupt()
		m.pool.Put(vm)
	}()

	config.set(vm)
	defer config.unset(vm)

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