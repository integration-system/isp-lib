package docs

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"text/template"

	"github.com/swaggo/swag"
)

type SwaggerInfo struct {
	Version     string
	Host        string
	BasePath    string
	Schemes     []string
	Title       string
	Description string
}

var (
	swaggerInfo = new(SwaggerInfo)
	lock        = new(sync.Mutex)
)

func InitSwagger(info SwaggerInfo, doc string) {
	lock.Lock()
	defer lock.Unlock()
	*swaggerInfo = info
	swaggerInfo.Description = strings.ReplaceAll(swaggerInfo.Description, "\n", "\\n")
	swag.Register(swag.Name, &swagProvider{doc: doc, sInfo: swaggerInfo, lock: lock})
}

func SetHost(host string) {
	lock.Lock()
	defer lock.Unlock()
	swaggerInfo.Host = host
}

type swagProvider struct {
	doc   string
	sInfo *SwaggerInfo
	lock  *sync.Mutex
}

func (s *swagProvider) ReadDoc() string {
	s.lock.Lock()
	defer s.lock.Unlock()

	t, err := template.New("swagger_info").Funcs(template.FuncMap{
		"marshal": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}).Parse(s.doc)
	if err != nil {
		return s.doc
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, s.sInfo); err != nil {
		return s.doc
	}

	return tpl.String()
}
