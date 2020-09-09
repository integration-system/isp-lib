package metric

import (
	"bytes"
	httpSwagger "github.com/swaggo/http-swagger"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"html/template"
	"path/filepath"
)

const indexTempl = `<!-- HTML for static distribution bundle build -->
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Swagger UI</title>
  <link href="https://fonts.googleapis.com/css?family=Open+Sans:400,700|Source+Code+Pro:300,600|Titillium+Web:400,600,700" rel="stylesheet">
  <link rel="stylesheet" type="text/css" href="./swagger-ui.css" >
  <link rel="icon" type="image/png" href="./favicon-32x32.png" sizes="32x32" />
  <link rel="icon" type="image/png" href="./favicon-16x16.png" sizes="16x16" />
  <style>
    html
    {
        box-sizing: border-box;
        overflow: -moz-scrollbars-vertical;
        overflow-y: scroll;
    }
    *,
    *:before,
    *:after
    {
        box-sizing: inherit;
    }

    body {
      margin:0;
      background: #fafafa;
    }
  </style>
</head>

<body>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" style="position:absolute;width:0;height:0">
  <defs>
    <symbol viewBox="0 0 20 20" id="unlocked">
      <path d="M15.8 8H14V5.6C14 2.703 12.665 1 10 1 7.334 1 6 2.703 6 5.6V6h2v-.801C8 3.754 8.797 3 10 3c1.203 0 2 .754 2 2.199V8H4c-.553 0-1 .646-1 1.199V17c0 .549.428 1.139.951 1.307l1.197.387C5.672 18.861 6.55 19 7.1 19h5.8c.549 0 1.428-.139 1.951-.307l1.196-.387c.524-.167.953-.757.953-1.306V9.199C17 8.646 16.352 8 15.8 8z"></path>
    </symbol>

    <symbol viewBox="0 0 20 20" id="locked">
      <path d="M15.8 8H14V5.6C14 2.703 12.665 1 10 1 7.334 1 6 2.703 6 5.6V8H4c-.553 0-1 .646-1 1.199V17c0 .549.428 1.139.951 1.307l1.197.387C5.672 18.861 6.55 19 7.1 19h5.8c.549 0 1.428-.139 1.951-.307l1.196-.387c.524-.167.953-.757.953-1.306V9.199C17 8.646 16.352 8 15.8 8zM12 8H8V5.199C8 3.754 8.797 3 10 3c1.203 0 2 .754 2 2.199V8z"/>
    </symbol>

    <symbol viewBox="0 0 20 20" id="close">
      <path d="M14.348 14.849c-.469.469-1.229.469-1.697 0L10 11.819l-2.651 3.029c-.469.469-1.229.469-1.697 0-.469-.469-.469-1.229 0-1.697l2.758-3.15-2.759-3.152c-.469-.469-.469-1.228 0-1.697.469-.469 1.228-.469 1.697 0L10 8.183l2.651-3.031c.469-.469 1.228-.469 1.697 0 .469.469.469 1.229 0 1.697l-2.758 3.152 2.758 3.15c.469.469.469 1.229 0 1.698z"/>
    </symbol>

    <symbol viewBox="0 0 20 20" id="large-arrow">
      <path d="M13.25 10L6.109 2.58c-.268-.27-.268-.707 0-.979.268-.27.701-.27.969 0l7.83 7.908c.268.271.268.709 0 .979l-7.83 7.908c-.268.271-.701.27-.969 0-.268-.269-.268-.707 0-.979L13.25 10z"/>
    </symbol>

    <symbol viewBox="0 0 20 20" id="large-arrow-down">
      <path d="M17.418 6.109c.272-.268.709-.268.979 0s.271.701 0 .969l-7.908 7.83c-.27.268-.707.268-.979 0l-7.908-7.83c-.27-.268-.27-.701 0-.969.271-.268.709-.268.979 0L10 13.25l7.418-7.141z"/>
    </symbol>

    <symbol viewBox="0 0 24 24" id="jump-to">
      <path d="M19 7v4H5.83l3.58-3.59L8 6l-6 6 6 6 1.41-1.41L5.83 13H21V7z"/>
    </symbol>

    <symbol viewBox="0 0 24 24" id="expand">
      <path d="M10 18h4v-2h-4v2zM3 6v2h18V6H3zm3 7h12v-2H6v2z"/>
    </symbol>
  </defs>
</svg>

<div id="swagger-ui"></div>

<script src="./swagger-ui-bundle.js"> </script>
<script src="./swagger-ui-standalone-preset.js"> </script>
<script>
var input = '<strong>Base host:</strong> <input id=baseHost style="border-radius:4px;border:1px solid #d9d9d9;padding:8px 10px">'
window.onload = async function() {
  // Build a system
 // enrich json
var url ="{{.URL}}";
var resp = await fetch(url)
	var spec = await resp.json();
	spec.securityDefinitions = {
		APPLICATION_TOKEN: {
            type: "apiKey",
			name: "X-APPLICATION-TOKEN",
            in: "header"
        },
		USER_TOKEN: {
            type: "apiKey",
            name: "X-USER-TOKEN",
            in: "header"
        },
		ADMIN_TOKEN: {
            type: "apiKey",
            name: "X-AUTH-ADMIN",
            in: "header"
        }}
Object.keys(spec.paths).forEach(path=>{
	var post = spec.paths[path].post
	post.security=[{APPLICATION_TOKEN:[]},{USER_TOKEN:[]},{ADMIN_TOKEN:[]}]
})
  const ui = SwaggerUIBundle({
	spec: spec,
    deepLinking: {{.DeepLinking}},
    docExpansion: "{{.DocExpansion}}",
    dom_id: "{{.DomID}}",
    validatorUrl: null,
	requestInterceptor: request	=> addedCustomHost(request,spec.host),
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout"
  })
window.ui = ui
setTimeout(createInput,1000,spec.host)
}

function createInput(host){
	var baseUrl = document.createElement('div');
	baseUrl.innerHTML = input;
	var parentWrapper = document.getElementsByClassName('schemes')[0];
	var firstChildWrapper = parentWrapper.firstChild;
	parentWrapper.insertBefore(baseUrl,firstChildWrapper)
	setInputValue(host)
}

function setInputValue(value){
	var input = document.getElementById('baseHost')
	input.value = value
}

function addedCustomHost(request,host){
	var input = document.getElementById('baseHost')
	if (input.value !=""){
		url = request.url.replace(host, input.value)
		request.url= url;
	}
	return request;
}
</script>
</body>

</html>
`

func makeSwaggerHandler(metricIp string, metricPort string) func(*fasthttp.RequestCtx) {
	config := &httpSwagger.Config{
		URL:          "doc.json",
		DeepLinking:  true,
		DocExpansion: "list",
		DomID:        "#swagger-ui",
	}
	configFunc := httpSwagger.URL(metricIp + ":" + metricPort + "/swagger/doc.json")
	configFunc(config)
	swaggerHandler := fasthttpadaptor.NewFastHTTPHandlerFunc(httpSwagger.Handler(configFunc))
	t := template.New("swagger_index.html")
	index, _ := t.Parse(indexTempl)

	return func(ctx *fasthttp.RequestCtx) {
		switch filepath.Ext(string(ctx.RequestURI())) {
		case ".html":
			ctx.SetContentType("text/html")
		case ".json":
			ctx.SetContentType("application/json")
		case ".css":
			ctx.SetContentType("text/css")
		case ".js":
			ctx.SetContentType("text/javascript")
		}

		if bytes.HasSuffix(ctx.RequestURI(), []byte("index.html")) {
			_ = index.Execute(ctx, config)
			return
		}
		swaggerHandler(ctx)
	}
}
