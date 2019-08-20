# {{GetText .Swagger.Info.Title}} v{{.Swagger.Info.Version}}

{{GetText .Swagger.Info.Description}}  
* {{GetText "Host"}} `{{.Swagger.Host}}`  
* {{GetText "Base Path"}} `{{.Swagger.BasePath}}`  

{{range $s := .Services}}
{{range $m := $s.Methods}}

## {{GetText $m.Operation.Summary}}

{{GetText $m.Operation.Description}}

```sh
curl -X {{$m.Request.Verb}} \
	{{$.Swagger.Host}}{{$m.Request.URI}} \
	-H 'Authorization: Bearer {{GetText "USE_YOUR_TOKEN"}}' {{if $m.Request.Example}}\
	-d '{{$m.Request.Example}}'
	{{end}}
```

{{AddSnippet $m.Name}}

### {{GetText "HTTP Request"}}

`{{$m.Request.Verb}} {{$.Swagger.Host}}{{$m.Request.URI}}`

{{if $m.Request.Query}}

### {{GetText "Query Parameters"}}

Name | Type | Description
---- | ---- | -----------
{{range $p := $m.Request.Query}}{{$p.Name}} | {{$p.Type.Name}} |{{GetText $p.Comment.Leading}}
{{end}}{{/* end request query range */}}
{{end}}{{/* end request query if*/}}

{{if $m.Request.Body}}
### {{GetText "Body Parameters"}}
{{template "message" $m.Request.Body}}
{{end}}{{/* end request body if*/}}

### {{GetText "Responses"}}

{{if $m.Response}}
#### {{GetText "Response body"}}
{{template "message" $m.Response}}

Example:

```json
{{$m.Response.Example}}
```
{{end}}{{/* end response if */}}

#### {{GetText "Response codes"}}
Status | Description
------ | -----------
{{range $k, $v := $m.Operation.Responses}}{{$k}} | {{GetText $v.Description}}
{{end}}{{/* end operation responses range */}}{{range $k, $v := $.Swagger.Responses}}{{$k}} | {{GetText $v.Description}}
{{end}}{{/* end swagger responses range */}}
{{end}}{{/* end methods range */}}
{{end}}{{/* end services range */}}

{{if .Enums}}
## {{GetText "Annex"}}

{{range $e := .Enums}}
#### {{$e.Name}}

{{GetText $e.Comment.Leading}}

Value | Description
----- | -----------
{{range $v := $e.Values}}{{$v.Name}} | {{GetText $v.Comment.Leading}}
{{end}}{{/* end enum values range */}}
{{end}}{{/* end enum range */}}
{{end}}{{/* end enum if */}}

{{- define "message"}}

Name | Type | Description
---- | ---- | -----------
{{range $f := .Fields}}{{$f.Name}} | {{$f.Type.Name}} | {{GetText $f.Comment.Leading}}
{{end}}{{/* end field range */}}

{{if .NestedMessages}}
##### {{GetText "Objects"}}

{{range $nm := .NestedMessages}}
###### {{$nm.Name}}

Name | Type | Description
---- | ---- | -----------
{{range $nf := $nm.Fields}}{{$nf.Name}} | {{$nf.Type.Name}} | {{GetText $nf.Comment.Leading}}
{{end}}{{/* end nested message field range */}}
{{end}}{{/* end nested message range*/}}
{{end}}{{/* end nested message if*/}}
{{end}}{{/* end message template*/}}
