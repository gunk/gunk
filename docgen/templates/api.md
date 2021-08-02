# {{GetText .Swagger.Info.Title}} v{{.Swagger.Info.Version}}

{{GetText .Swagger.Info.Description}}

* {{GetText "Host"}} `{{$.SwaggerScheme}}{{.Swagger.Host}}`

* {{GetText "Base Path"}} `{{.Swagger.BasePath}}`
{{- range $s := .Services}}
{{- range $m := $s.Methods}}

## {{GetText $m.Operation.Summary}} {{CustomHeaderId $m.HeaderID}}

{{GetText $m.Operation.Description}}

```sh
curl -X {{$m.Request.Verb}} \
	{{$.SwaggerScheme}}{{$.Swagger.Host}}{{$.Swagger.BasePath}}{{$m.Request.URI}} \
	-H 'x-api-key: {{GetText "USE_YOUR_API_KEY"}}'{{if $m.Request.Example}} \
	-d '{{$m.Request.Example}}'{{end}}
```

{{AddSnippet $m.Name}}

### {{GetText "HTTP Request"}} {{CustomHeaderId "http-request-" $m.HeaderID}}

`{{$m.Request.Verb}} {{$.SwaggerScheme}}{{$.Swagger.Host}}{{$.Swagger.BasePath}}{{$m.Request.URI}}`

{{if $m.Request.Query}}

### {{GetText "Query Parameters"}} {{CustomHeaderId "query-parameters-" $m.HeaderID}}

Name | Type | Description
---- | ---- | -----------
{{range $p := $m.Request.Query}}{{mdType $p.JSONName}} | {{mdType $p.Type.Name}} |{{GetText $p.Comment.Leading}}
{{end}}{{/* end request query range */}}
{{end}}{{/* end request query if*/}}

{{if $m.Request.Body}}
### {{GetText "Body Parameters"}} {{CustomHeaderId "body-parameters-" $m.HeaderID}}
{{template "message" $m.Request.Body}}
{{end}}{{/* end request body if*/}}

### {{GetText "Responses"}} {{CustomHeaderId "responses-" $m.HeaderID}}

{{if $m.Response}}

#### {{GetText "Response body"}} {{CustomHeaderId "response-body-" $m.HeaderID}}
{{template "message" $m.Response}}

Example:

```json
{{$m.Response.Example}}
```
{{end}}{{/* end response if */}}

#### {{GetText "Response codes"}} {{CustomHeaderId "response-codes-" $m.HeaderID}}

Status | Description
------ | -----------
{{range $k, $v := $m.Operation.Responses}}{{$k}} | {{GetText $v.Description}}
{{end}}{{/* end operation responses range */}}{{range $k, $v := $.Swagger.Responses}}{{$k}} | {{GetText $v.Description}}
{{end}}{{/* end swagger responses range */}}
{{end}}{{/* end methods range */}}
{{end}}{{/* end services range */}}

{{- define "message"}}

Name | Type | Description
---- | ---- | -----------
{{range $f := .Fields}}{{if ne $f.JSONName "-"}}{{mdType $f.JSONName}} | {{mdType $f.Type.Name}} | {{GetText $f.Comment.Leading}}{{end}}
{{end}}{{/* end field range */}}

{{if .NestedMessages}}
##### {{GetText "Objects"}} {{CustomHeaderId "objects-" .Name}}

{{range $nm := .NestedMessages}}
###### {{$nm.Name}}

Name | Type | Description
---- | ---- | -----------
{{range $nf := $nm.Fields}}{{if ne $nf.JSONName "-"}}{{mdType $nf.JSONName}} | {{mdType $nf.Type.Name}} | {{GetText $nf.Comment.Leading}}{{end}}
{{end}}{{/* end nested message field range */}}
{{end}}{{/* end nested message range*/}}
{{end}}{{/* end nested message if*/}}


{{if .Enums}}
##### {{GetText "Enums"}} {{CustomHeaderId "enums-" .Name}}

{{range $e := .Enums}}
###### {{$e.Name}}

{{GetText $e.Comment.Leading}}

Value | Description
----- | -----------
{{range $v := $e.Values}}{{$v.Name}} | {{GetText $v.Comment.Leading}}
{{end}}{{/* end enum values range */}}
{{end}}{{/* end enum range*/}}
{{end}}{{/* end enum if*/}}

{{end}}{{/* end message template*/}}
