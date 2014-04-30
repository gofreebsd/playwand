package {{.Name}}

import (
	"fmt"

	"github.com/vasiliyl/playwand/proto"
)

type ServerObjectsFactory struct {
	c *proto.Conn
}

func NewServer(c *proto.Conn) ServerObjectsFactory {
	return ServerObjectsFactory{c}
}

{{range .Interfaces}}
{{$interfaceName := Exported .Name}}

type {{$interfaceName}}EventsInterface interface {
	{{range .Events}}
	{{Exported .Name}}({{Exported .Interface .Name .Kind}}) error
	{{end}}
}


type Server{{$interfaceName}} struct {
	c *proto.Conn
	id proto.ObjectId
	i {{$interfaceName}}EventsInterface
}

func (c ServerObjectsFactory) New{{$interfaceName}}(i {{$interfaceName}}EventsInterface) Server{{$interfaceName}} {
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


func (o Server{{$interfaceName}}) Handle(opcode uint16, c *proto.Conn) error {
	switch opcode {
		{{range .Events}}
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

{{range .Requests}}
func (o Server{{$interfaceName}}) {{Exported .Name}}({{range .Args}}{{Unexported .Name}} {{GoType .Type}}, {{end}}) {{Exported .Interface .Name .Kind}} {
	return {{Exported .Interface .Name .Kind}}{ 
		id: o.id,
		{{range .Args}}
		{{Exported .Name}}: {{Unexported .Name}},
		{{end}} }
}

func (o Server{{$interfaceName}}) New{{Exported .Name}}() {{Exported .Interface .Name .Kind}} {
	return {{Exported .Interface .Name .Kind}}{id: o.id}
}
{{end}}
{{end}}
