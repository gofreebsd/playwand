package {{.Name}}

import (
	"fmt"

	"github.com/vasiliyl/playwand/proto"
)

type Client struct {
	c *proto.Conn
}

func NewClient(c *proto.Conn) Client {
	return Client{c}
}

type Server struct {
	c *proto.Conn
}

func NewServer(c *proto.Conn) Server {
	return Server{c}
}

{{range .Interfaces}}

{{Exported .Name | Comment}}
{{Comment .Description}}

{{$iname := .Name}}
{{range .Enums}}
{{$ename := .Name}}
{{Comment .Description}}
const (
{{range .Entries}}
	{{Comment .Summary}}
	{{Const $iname $ename .Name}} = {{.Value}}
{{end}}
)
{{end}}

{{$interfaceName := Exported .Name}}

{{/* CLIENT */}}
type Client{{$interfaceName}}Implementation interface {
	{{range .Events}}
	{{Comment .Description}}
	{{Exported .Name}}({{range .Args}}{{Unexported .Name}} {{GoType .Type}},{{end}}) error
	{{end}}
}


type Client{{$interfaceName}} struct {
	c *proto.Conn
	id proto.ObjectId
	i Client{{$interfaceName}}Implementation
}

func (c Client) New{{$interfaceName}}(i Client{{$interfaceName}}Implementation) Client{{$interfaceName}} {
	o := Client{{$interfaceName}}{
		c: c.c,
		id: c.c.NextId(),
		i: i,
	}
	c.c.AddObject(o.id, o)
	return o
}

func (o Client{{$interfaceName}}) Id() proto.ObjectId {
	return o.id
}

func (o Client{{$interfaceName}}) Handle(m *proto.Message) (err error) {
	switch m.Opcode() {
		{{range .Events}}
	case {{.Opcode}}:
		var ({{range .Args}}
			{{Unexported .Name}} {{GoType .Type}}
		{{end}})

		{{range .Args}}
		if {{Unexported .Name}}, err = m.Read{{WlType .Type}}(); err != nil {
			return
		}
		{{end}}
		return o.i.{{Exported .Name}}({{range .Args}}{{Unexported .Name}},{{end}})
		{{end}}

	default:
		return fmt.Errorf("{{$interfaceName}}: invalid event opcode: %s", m.Opcode())
	}
}

{{range .Requests}}
{{Comment .Description}}
func (o Client{{$interfaceName}}) {{Exported .Name}}({{range .Args}}{{Unexported .Name}} {{GoType .Type}}, {{end}}) error {
	m := proto.NewMessage(o.id, {{.Opcode}})
	{{range .Args}}
	if err := m.Write{{WlType .Type}}({{Unexported .Name}}); err != nil {
		return err
	}
	{{end}}
	return o.c.WriteMessage(m)
}
{{end}}

{{/* SERVER */}}
type Server{{$interfaceName}}Implementation interface {
	{{range .Requests}}
	{{Comment .Description}}
	{{Exported .Name}}({{range .Args}}{{Unexported .Name}} {{GoType .Type}},{{end}}) error
	{{end}}
}


type Server{{$interfaceName}} struct {
	c *proto.Conn
	id proto.ObjectId
	i Server{{$interfaceName}}Implementation
}

func (c Server) New{{$interfaceName}}(i Server{{$interfaceName}}Implementation) Server{{$interfaceName}} {
	o := Server{{$interfaceName}}{
		c: c.c,
		id: c.c.NextId(),
		i: i,
	}
	c.c.AddObject(o.id, o)
	return o
}

func (o Server{{$interfaceName}}) Id() proto.ObjectId {
	return o.id
}

func (o Server{{$interfaceName}}) Handle(m *proto.Message) (err error) {
	switch m.Opcode() {
		{{range .Requests}}
	case {{.Opcode}}:
		var ({{range .Args}}
			{{Unexported .Name}} {{GoType .Type}}
		{{end}})

		{{range .Args}}
		if {{Unexported .Name}}, err = m.Read{{WlType .Type}}(); err != nil {
			return
		}
		{{end}}
		return o.i.{{Exported .Name}}({{range .Args}}{{Unexported .Name}},{{end}})
		{{end}}

	default:
		return fmt.Errorf("{{$interfaceName}}: invalid request opcode: %s", m.Opcode())
	}
}

{{range .Events}}
{{Comment .Description}}
func (o Server{{$interfaceName}}) {{Exported .Name}}({{range .Args}}{{Unexported .Name}} {{GoType .Type}}, {{end}}) error {
	m := proto.NewMessage(o.id, {{.Opcode}})
	{{range .Args}}
	if err := m.Write{{WlType .Type}}({{Unexported .Name}}); err != nil {
		return err
	}
	{{end}}
	return o.c.WriteMessage(m)
}
{{end}}

{{end}}
