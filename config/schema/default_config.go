package schema

import (
	"encoding/json"
	"os"
	p "path"

	"github.com/integration-system/isp-lib/v2/utils"
)

func ExtractConfig(path string) (map[string]interface{}, error) {
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := make(map[string]interface{})
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}

func ResolveDefaultConfigPath(path string) string {
	if p.IsAbs(path) {
		return path
	}
	if utils.DEV {
		return p.Join("conf", path)
	} else {
		ex, _ := os.Executable()
		dir := p.Dir(ex)
		return p.Join(dir, path)
	}
}
