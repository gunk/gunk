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
{{end}}{{end}}
{{if $m.Request.Body}}
### {{GetText "Body Parameters"}}

Name | Type | Description
---- | ---- | -----------
{{range $p := $m.Request.Body.Fields}}{{$p.Name}} | {{$p.Type.Name}} |{{GetText $p.Comment.Leading}}
{{end}}{{end}}
### {{GetText "Responses"}}
{{if $m.Response}}
#### {{GetText "Response body"}}

Name | Type | Description
---- | ---- | -----------
{{range $f := $m.Response.Fields}}{{$f.Name}} | {{$f.Type.Name}} | {{GetText $f.Comment.Leading}}
{{end}}{{end}}

<!-- TODO: add example -->

#### {{GetText "Response codes"}}
Status | Description
------ | -----------
{{range $k, $v := $m.Operation.Responses}}{{$k}} | {{GetText $v.Description}}
{{end}}{{range $k, $v := $.Swagger.Responses}}{{$k}} | {{GetText $v.Description}}
{{end}}{{end}}{{end}}

## {{GetText "Annex"}}

{{range $e := .Enums}}

#### {{$e.Name}}

{{GetText $e.Comment.Leading}}

Value | Description
----- | -----------
{{range $v := $e.Values}}{{$v.Name}} | {{GetText $v.Comment.Leading}}
{{end}}{{end}}
