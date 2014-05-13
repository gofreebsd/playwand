// DO NOT EDIT THIS FILE

package {{.Name}}

import (
	"github.com/vasiliyl/playwand/proto"
)


{{define "message"}}

{{$exportedMessageStructName := Exported .Interface .Name .Kind}}


type {{$exportedMessageStructName}} struct {
	{{range .Args}}
	{{Exported .Name}} {{GoType .Type}}
	{{end}}
}

func (m *{{$exportedMessageStructName}}) Unmarshal(wm *proto.Message) (err error) {
	{{range .Args}}
	if m.{{Exported .Name}}, err = wm.Read{{WlType .Type}}(); err != nil {
		return
	}
	{{end}}
	return nil
}

func (m {{$exportedMessageStructName}}) Marshal(wm *proto.Message) (err error) {
	{{range .Args}}
	if err = wm.Write{{WlType .Type}}(m.{{Exported .Name}}); err != nil {
		return
	}
	{{end}}
	return nil
}
{{end}}

{{range .Interfaces}}


{{range .Requests}}
{{template "message" .}}
{{end}}
{{range .Events}}
{{template "message" .}}
{{end}}

{{end}}

