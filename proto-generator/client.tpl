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


{{range .Interfaces}}

{{$iname := .Name}}
{{range .Enums}}
{{$ename := .Name}}
type {{Exported $iname $ename}} uint32
const (
{{range .Entries}}
	{{Const $iname $ename .Name}} {{Exported $iname $ename}} = {{.Value}}
{{end}}
)
{{end}}

{{$interfaceName := Exported .Name}}

type Client{{$interfaceName}}Implementation interface {
	{{range .Events}}
	{{Exported .Name}}({{Exported .Interface .Name .Kind}}) error
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

func (o Client{{$interfaceName}}) Handle(m *proto.Message) error {
	switch m.Opcode() {
		{{range .Events}}
	case {{.Opcode}}:
		var sm {{Exported .Interface .Name .Kind}}
		if err := sm.Unmarshal(m); err != nil {
			return err
		}
		return o.i.{{Exported .Name}}(sm)
		{{end}}

	default:
		return fmt.Errorf("{{Exported .Name}}: invalid event opcode: %s", m.Opcode())
	}
}

{{range .Requests}}
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

{{end}}
