package {{.Name}}

import (
	"fmt"

	"github.com/vasiliyl/playwand/proto"
)

type ClientObjectsFactory struct {
	c *proto.Conn
}

func NewClient(c *proto.Conn) ClientObjectsFactory {
	return ClientObjectsFactory{c}
}

{{range .Interfaces}}
{{$interfaceName := Exported .Name}}

type {{$interfaceName}}RequestsInterface interface {
	{{range .Requests}}
	{{Exported .Name}}({{Exported .Interface .Name .Kind}}) error
	{{end}}
}


type Client{{$interfaceName}} struct {
	c *proto.Conn
	id proto.ObjectId
	i {{$interfaceName}}RequestsInterface
}

func (c ClientObjectsFactory) New{{$interfaceName}}(i {{$interfaceName}}RequestsInterface) Client{{$interfaceName}} {
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

func (o Client{{$interfaceName}}) Handle(opcode uint16, c *proto.Conn) error {
	switch opcode {
		{{range .Requests}}
	case {{.Opcode}}:
		m := {{Exported .Interface .Name .Kind}}{id: o.id}
		if err := m.readFrom(c); err != nil {
			return err
		}
		return o.i.{{Exported .Name}}(m)
		{{end}}

	default:
		return fmt.Errorf("{{Exported .Name}}: invalid opcode: %s", opcode)
	}
}

{{range .Events}}
func (o Client{{$interfaceName}}) {{Exported .Name}}({{range .Args}}{{Unexported .Name}} {{GoType .Type}}, {{end}}) {{Exported .Interface .Name .Kind}} {
	return {{Exported .Interface .Name .Kind}}{ 
		id: o.id,
		{{range .Args}}
		{{Exported .Name}}: {{Unexported .Name}},
		{{end}} }
}

func (o Client{{$interfaceName}}) New{{Exported .Name}}() {{Exported .Interface .Name .Kind}} {
	return {{Exported .Interface .Name .Kind}}{id: o.id}
}
{{end}}
{{end}}
