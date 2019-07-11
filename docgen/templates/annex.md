# {{GetText "Annex"}}

{{range $m := .Messages}}

## {{$m.Name}}

{{GetText $m.Comment.Leading}}

Name | Type | Description
---- | ---- | -----------
{{range $f := $m.Fields}}{{$f.Name}} | {{$f.Type.Name}} | {{GetText $f.Comment.Leading}}
{{end}}{{end}}

{{range $e := .Enums}}

## {{$e.Name}}

{{GetText $e.Comment.Leading}}

Value | Description
----- | -----------
{{range $v := $e.Values}}{{$v.Name}} | {{GetText $v.Comment.Leading}}
{{end}}{{end}}
