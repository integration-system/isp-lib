package rc_test

import (
	"testing"

	"github.com/integration-system/isp-lib/v3/json"
	"github.com/integration-system/isp-lib/v3/rc"
	"github.com/stretchr/testify/require"
)

type noneValidation struct {
}

func (n noneValidation) ValidateToError(value interface{}) error {
	return nil
}

func TestConfig_Upgrade(t *testing.T) {
	require := require.New(t)

	override := `{"a": {"a": 1}}`
	config := rc.New(noneValidation{}, json.RawMessage(override))

	type cfgType struct {
		A struct {
			A int
		}
		B int
	}

	cfg1 := `{"a": {"a": 2}, "b": 2}`
	expectedNewCfg := cfgType{
		A: struct {
			A int
		}{1},
		B: 2,
	}
	expectedPrevCfg := cfgType{}
	newCfg := cfgType{}
	prevCfg := cfgType{}
	err := config.Upgrade(json.RawMessage(cfg1), &newCfg, &prevCfg)
	require.NoError(err)
	require.EqualValues(expectedNewCfg, newCfg)
	require.EqualValues(expectedPrevCfg, prevCfg)

	cfg2 := `{"a": {"a": 4}, "b": 3}`
	expectedNewCfg = cfgType{
		A: struct {
			A int
		}{1},
		B: 3,
	}
	expectedPrevCfg = cfgType{
		A: struct {
			A int
		}{1},
		B: 2,
	}
	newCfg = cfgType{}
	prevCfg = cfgType{}
	err = config.Upgrade(json.RawMessage(cfg2), &newCfg, &prevCfg)
	require.NoError(err)
	require.EqualValues(expectedNewCfg, newCfg)
	require.EqualValues(expectedPrevCfg, prevCfg)
}
