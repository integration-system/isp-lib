package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoteConfigOverride(t *testing.T) {
	assert := assert.New(t)

	type Anon struct {
		V string
	}

	type Inner struct {
		CamelCase string
		B         string
		Anon
	}

	type Config struct {
		A string
		B int
		C bool
		D Inner
	}

	original := Config{A: "test1", B: 1, C: true, D: Inner{CamelCase: "test1", B: "test2", Anon: Anon{V: "test4"}}}

	expect := Config{A: "test2", B: 2, C: false, D: Inner{CamelCase: "test2", B: "test2", Anon: Anon{V: "test5"}}}

	bytes, err := json.Marshal(original)
	if err != nil {
		panic(err)
	}

	assert.Nil(os.Setenv(RemoteConfigEnvPrefix+"_A", "test2#{string}"))
	assert.Nil(os.Setenv(RemoteConfigEnvPrefix+"_B", "2#{int}"))
	assert.Nil(os.Setenv(RemoteConfigEnvPrefix+"_C", "false#{bool}"))
	assert.Nil(os.Setenv(RemoteConfigEnvPrefix+"_D.CAMELCASE", "test2#{string}"))
	assert.Nil(os.Setenv(RemoteConfigEnvPrefix+"_D.V", "test5#{string}"))

	ptr, err := InitRemoteConfig(&original, bytes)
	if err != nil {
		t.Error(err)
	}
	original = *ptr.(*Config)

	assert.Equal(expect, original)
}

func TestInitRemoteConfig(t *testing.T) {
	assert := assert.New(t)

	oldConfig, expectedConfig := getFirstConfData()
	remoteConfig, err := json.Marshal(expectedConfig)
	assert.NoError(err)
	ptr, err := InitRemoteConfig(oldConfig, remoteConfig)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(expectedConfig, ptr)

	secOldConf, secExpConf := getSecondConfData()
	secRemoteConf, err := json.Marshal(secExpConf)
	assert.NoError(err)
	ptr, err = InitRemoteConfig(secOldConf, secRemoteConf)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(secExpConf, ptr)
}

func getFirstConfData() (oldConfig, newConfig interface{}) {
	type supStructure struct {
		Integer int
		Varchar string
	}
	type config struct {
		Integer               int
		Varchar               string
		SupStructure          supStructure
		MapStringSupStructure map[string]supStructure
	}
	oldConfig = &config{
		Integer:      1,
		Varchar:      "one",
		SupStructure: supStructure{Integer: 1, Varchar: "one"},
		MapStringSupStructure: map[string]supStructure{
			"one":   {Integer: 1, Varchar: "one"},
			"two":   {Integer: 2, Varchar: "two"},
			"three": {Integer: 3, Varchar: "three"},
		},
	}
	newConfig = &config{
		Integer: 2,
		Varchar: "two",
		MapStringSupStructure: map[string]supStructure{
			"two":   {Integer: 4, Varchar: "four"},
			"three": {Integer: 3, Varchar: "three"},
		},
	}
	return oldConfig, newConfig
}

func getSecondConfData() (oldConfig, newConfig interface{}) {
	type config struct {
		Integer int
		Varchar string
	}
	oldConfig = &config{
		Integer: 1,
	}
	newConfig = &config{
		Varchar: "one",
	}
	return oldConfig, newConfig
}
