// Code generated by regen.go. DO NOT EDIT.

package htmlstack

import (
	"html/template"
)

const indexHTML = "<!DOCTYPE html>\n{{- /* Accepts a Args */ -}}\n{{- define \"RenderArgs\" -}}\n<span class=\"args\"><span>\n{{- $elided := .Elided -}}\n{{- if .Processed -}}\n{{- $l := len .Processed -}}\n{{- $last := minus $l 1 -}}\n{{- range $i, $e := .Processed -}}\n{{- $e -}}\n{{- $isNotLast := ne $i $last -}}\n{{- if or $elided $isNotLast}}, {{end -}}\n{{- end -}}\n{{- else -}}\n{{- $l := len .Values -}}\n{{- $last := minus $l 1 -}}\n{{- range $i, $e := .Values -}}\n{{- $e.String -}}\n{{- $isNotLast := ne $i $last -}}\n{{- if or $elided $isNotLast}}, {{end -}}\n{{- end -}}\n{{- end -}}\n{{- if $elided}}…{{end -}}\n</span></span>\n{{- end -}}\n{{- /* Accepts a Call */ -}}\n{{- define \"RenderCall\" -}}\n<span class=\"call\"><a href=\"{{srcURL .}}\">{{.SrcName}}:{{.Line}}</a> <span class=\"{{funcClass .}}\">\n<a href=\"{{pkgURL .}}\">{{.Func.PkgName}}.{{.Func.Name}}</a></span>({{template \"RenderArgs\" .Args}})</span>\n{{- if isDebug -}}\n<br>SrcPath: {{.SrcPath}}\n<br>LocalSrcPath: {{.LocalSrcPath}}\n<br>Func: {{.Func.Raw}}\n<br>IsStdlib: {{.IsStdlib}}\n{{- end -}}\n{{- end -}}\n{{- /* Accepts a Stack */ -}}\n{{- define \"RenderCalls\" -}}\n<table class=\"stack\">\n{{- range $i, $e := .Calls -}}\n<tr>\n<td>{{$i}}</td>\n<td>\n<a href=\"{{pkgURL $e}}\">{{$e.Func.PkgName}}</a>\n</td>\n<td>\n<a href=\"{{srcURL $e}}\">{{$e.SrcName}}:{{$e.Line}}</a>\n</td>\n<td>\n<span class=\"{{funcClass $e}}\"><a href=\"{{pkgURL $e}}\">{{$e.Func.Name}}</a></span>({{template \"RenderArgs\" $e.Args}})\n</td>\n</tr>\n{{- end -}}\n{{- if .Elided}}<tr><td>(…)</td><tr>{{end -}}\n</table>\n{{- end -}}\n<meta charset=\"UTF-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>PanicParse</title>\n<link rel=\"shortcut icon\" type=\"image/gif\" href=\"data:image/gif;base64,{{.Favicon}}\"/>\n<style>\n{{- /* Minimal CSS reset */ -}}\n* {\nfont-family: inherit;\nfont-size: 1em;\nmargin: 0;\npadding: 0;\n}\nhtml {\nbox-sizing: border-box;\nfont-size: 62.5%;\n}\n*, *:before, *:after {\nbox-sizing: inherit;\n}\nh1 {\nfont-size: 1.5em;\nmargin-bottom: 0.2em;\nmargin-top: 0.5em;\n}\nh2 {\nfont-size: 1.2em;\nmargin-bottom: 0.2em;\nmargin-top: 0.3em;\n}\nbody {\nfont-size: 1.6em;\nmargin: 2px;\n}\nli {\nmargin-left: 2.5em;\n}\na {\ncolor: inherit;\ntext-decoration: inherit;\n}\nol, ul {\nmargin-bottom: 0.5em;\nmargin-top: 0.5em;\n}\np {\nmargin-bottom: 2em;\n}\ntable.stack {\nmargin: 0.6em;\n}\ntable.stack tr:hover {\nbackground-color: #DDD;\n}\ntable.stack td {\nfont-family: monospace;\npadding: 0.2em 0.4em 0.2em;\n}\n.call {\nfont-family: monospace;\n}\n@media screen and (max-width: 500px) {\nh1 {\nfont-size: 1.3em;\n}\n}\n@media screen and (max-width: 500px) and (orientation: portrait) {\n.args span {\ndisplay: none;\n}\n.args::after {\ncontent: '…';\n}\n}\n.created {\nwhite-space: nowrap;\n}\n.topright {\nfloat: right;\n}\n.button {\nbackground-color: white;\nborder: 2px solid #4CAF50;\ncolor: black;\nmargin: 0.3em;\npadding: 0.6em 1.0em;\ntransition-duration: 0.4s;\n}\n.button:hover {\nbackground-color: #4CAF50;\ncolor: white;\nbox-shadow: 0 12px 16px 0 rgba(0,0,0,0.24), 0 17px 50px 0 rgba(0,0,0,0.19);\n}\n#augment {\ndisplay: none;\n}\n#content {\nwidth: 100%;\n}\n{{- /* Highlights */ -}}\n.FuncStdLibExported {\ncolor: #00B000;\n}\n.FuncStdLib {\ncolor: #006000;\n}\n.FuncMain {\ncolor: #808000;\n}\n.FuncOtherExported {\ncolor: #C00000;\n}\n.FuncOther {\ncolor: #800000;\n}\n.RoutineFirst {\n}\n.Routine {\n}\n</style>\n<script>\nfunction getParamByName(name) {\nlet query = window.location.search.substring(1);\nlet vars = query.split(\"&\");\nfor (let i=0; i<vars.length; i++) {\nlet pair = vars[i].split(\"=\");\nif (pair[0] == name) {\nreturn pair[1];\n}\n}\n}\nfunction ready() {\nif (getParamByName(\"augment\") === undefined) {\ndocument.getElementById(\"augment\").style.display = \"inline\";\n}\n}\ndocument.addEventListener(\"DOMContentLoaded\", ready);\n</script>\n<div id=\"content\">\n<div class=\"topright\">\n{{- /* Only shown when augment query parameter is not specified */ -}}\n<a class=button id=augment href=\"?augment=1\">Analyse sources</a>\n</div>\n{{- range $i, $e := .Buckets -}}\n{{$l := len $e.IDs}}\n<h1>Signature #{{$i}}: <span class=\"{{routineClass $e}}\">{{$l}} routine{{if ne 1 $l}}s{{end}}: <span class=\"state\">{{$e.State}}</span>\n{{- if $e.SleepMax -}}\n{{- if ne $e.SleepMin $e.SleepMax}} <span class=\"sleep\">[{{$e.SleepMin}}~{{$e.SleepMax}} mins]</span>\n{{- else}} <span class=\"sleep\">[{{$e.SleepMax}} mins]</span>\n{{- end -}}\n{{- end -}}\n</h1>\n{{if $e.Locked}} <span class=\"locked\">[locked]</span>\n{{- end -}}\n{{- if $e.CreatedBy.Func.Raw}} <span class=\"created\">Created by: {{template \"RenderCall\" $e.CreatedBy}}</span>\n{{- end -}}\n{{template \"RenderCalls\" $e.Signature.Stack}}\n{{- end -}}\n</div>\n<p>\n<div id=\"legend\">\nCreated on {{.Now.String}}:\n<ul>\n<li>{{.Version}}</li>\n<li>GOROOT: {{.GOROOT}}</li>\n<li>GOPATH: {{.GOPATH}}</li>\n<li>GOMAXPROCS: {{.GOMAXPROCS}}</li>\n{{- if .NeedsEnv -}}\n<li>To see all goroutines, visit <a\nhref=https://github.com/maruel/panicparse#gotraceback>github.com/maruel/panicparse</a></li>\n{{- end -}}\n</ul>\n</div>\n"

// favicon is the bomb emoji U+1F4A3 in Noto Emoji as a 128x128 base64 encoded
// PNG.
//
// See README.md for license and how to retrieve it.
const favicon template.HTML = "R0lGODlhQABAAMZhACEhISIiIiMjIyQkJCUlJSYmJicnJygoKCkpKSoqKisrKywsLC0tLS4uLi8vLzAwMDExMTIyMjMzMzQ0NDU1NTY2Njc3Nzg4ODk5OTo6Ojs7Ozw8PD09PT4+Pj8/P0BAQEFBQUJCQkNDQ0REREVFRUZGRkdHR4o1MEhISElJSUpKSktLS0xMTE1NTU5OTk9PT1BQUFFRUVJSUv8fQFNTU1RUVKc+N1VVVVZWVldXV/8mPVhYWFlZWf8oPP8pO1paWltbW1xcXF1dXV5eXl9fX2BgYGFhYf80NWJiYv8+MP9ALvRDNv9JKcxYUfRPQ/9PJvRTSP9VI/9aIPRcUf9dHudhV/9lGv91EP91Ef96Dv99C/9+DP+FB/+MA/+PAf+QAP+RAP///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////yH5BAEKAH8ALAAAAABAAEAAAAf+gH+Cg4SFhoeIiYqLjI2Oj5CRjSkjJiqSkl6YfyMXCQCgBRUkm45ahF+NJRIBAAEKEhIKAAIepYVgg0uCVINXjCocAwAEGSWEHwIBI7eoXIRHgklSiyQPrhcohx0AEM2FVkqFM4sfBQALIokqBAKX34PijigVrhYpiwwAzN+7g1NOmqzrYACAgRCNFgAwAa8QlCUnQLwTpGKEhgP2tAEbQGBiMyxPdPypYgPUAAYRJDA450oCv0YfAESAl6tQig4MBIACFYCBhmOKVqh4p8IBAFsNB/kgVBEEiBEeE7loYSLF0BQYJEQt5e8PE0E9MMmAocGAAQgdSgwdmvRPlkH+USThoKFhgQIEBtopwAB1a6kug7ZAAsLjRgYIDxowuFtgQCutDWtCEkJZSI4NFShIiJB4AV7HAVD4bWuoiGkiOWSo6KAhAwYLmiE4WJAgbwAPo0kLKmKkSA4VIZx++OCBwwbXFiZEcMCg9oAJudvyNlKDxIgRJLKPEBH8Q4fjsJc3N4CArW5CRZD8MEGihAkUKFLAN1GChAgQxTNcoLDccwES0X1jBBIwkFCVCiuwoKCCQqFQwgghfMCBfvzNdgAHQp33R28rlCAaCy68AMOIMLzwQgssqIACCRFOmBwEDSSAQF+6mZaChyu08EIMMtBQw480yBDDiSus2CIGFUj+8IBnBogQoCS83YjCCi7AQMMNOOSww5Y54FDDWC0UyaIHG+wHY20EAJgUEUawQMKULsRAAw47/ABEEHgStsMNNMDQggomjABCBxlYIIGFBBwgWkNF3CCCJS1YiQMPQAhBBBFFEDGEEEDsMJefKpQgApkXTAABAwgUEIBWTzKyqQhvsvDCnDwEMcR0RuRaxBCd3iDDC0UKygGSEcRogE4SNRMEES2IoFYLMdywAxBD5GqttbsCkUMNfqbAIqGGOqDAAcMkYN4mnH7w6AovyIDDD0Lgeq2uQhQmgwuAjqpBckuSCwputwjhwgcjoMCClTkAwea88w7xw1zAmiDCB2X+TvAAqgS00sC5kVT6AQixwlDDDssyPC8R2tLwAgsojEBxqacikDEoam3yww0cgJzCwSMHIa/JKOfQJ8sul0lBzDMDsEGrh/zQQs4hj6ywydcGrXKwL5uKcSvdMG3IDyNsQLDBsyZcLdW5OozDrytInPWSMnNtAMeO5MDDBmKv64K7PMSLdhFBFBYDvqJ6sK+SC/i706KRTGqBBh6E8GwMUt9qchFC/LBtt99mkKS4Bgyzk5qR3HBDBRlATbYM0lL7MxGZ73lv2y4Py5+xOu2UbCRfUnBB3s9a2bqtl2oaxA+yr3xjCIaXenHcOwGwOyQ/SmBBBh08BanwdtsJBBD+P/CQg68nAlq754cmLvpOTkoiQw0S+L5B5G/mCMP7WOagPw43fFl+oIPSAMxwFz0A1EwSNDAU6jjwgRC8SQU6uh8NJhikGMDABSliD/M2gKT0HSBpPKGbI2TAAQhM4HoMdGBVchQiE5kIgytIQaA2WCiLjSd3O1GA1wpRIAdEgAIojNwIPGSVFSTIiCqQ4YPwAx5TNWBcBCggACqwQ0JgcAE+BGIG5gcCEWDHPfBBAX2sE6Hv1LA/qcLhTj5QxUHISiVZtAAGNsABD4AgBCLw4nXyCALicEADr1HObNIoRQJYpRQtAEECGODDCVTgAlvkQAc8MJzheMADHfijfir+YKpBhk6KU2wjIYyogARgEQLxs8AFMJABDeDtla3BwAU4qaQGMGl90QuAJZqxAoIgQAENeAAqHWkBVV7gmBewQGYmIAHZLCZVuIweFb8xlJUcwJQNcAAEUjKBblKgm7HgjANsmYADFECN0SMA45pREY6Qx5QMyOYDhAkBxDxgnAygzQHygs7oAawhKthAANx5AAQkQAELWAADFprQUiJgn+0A5U6oKEpEDIUCrhDAAApgloIi4KMPNUABCDAAZUgUFBA4ZFsuupMAaJQjBIgpSUvKtZN2Q6WkGcoFasqTAPiUpzYFAAVwqpuhmCOoQR1AB0SYUxVVAKhIBUUE1FI6UUkMhQQV6KdEV+WkqpZiKCgoYRRBOYAHcMASTNWQIdYSKhBw4K0MBNBa1GpVtrJ1BXTNq173qohAAAA7"
