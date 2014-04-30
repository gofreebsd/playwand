package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	//"go/parser"
	//"go/printer"
	//"go/token"
	"log"
	"os"
	"strings"
	"text/template"
)

type Protocol struct {
	Name       string      `xml:"name,attr"`
	Interfaces []Interface `xml:"interface"`
}

func (p Protocol) analyze() {
	for i := range p.Interfaces {
		p.Interfaces[i].analyze()
	}
}

type Interface struct {
	Name        string `xml:"name,attr"`
	Version     int    `xml:"version,attr"`
	Description string `xml:"description"`

	Requests []Message `xml:"request"`
	Events   []Message `xml:"event"`
	//Errors   []Error   `xml:"error"`
}

func (i *Interface) analyze() {
	if strings.HasPrefix(i.Name, "wl_") {
		i.Name = i.Name[3:]
	}
	for j := range i.Requests {
		i.Requests[j].Kind = "Request"
		i.Requests[j].Opcode = uint16(j)
		i.Requests[j].Interface = i.Name
	}
	for j := range i.Events {
		i.Events[j].Kind = "Event"
		i.Events[j].Opcode = uint16(j)
		i.Events[j].Interface = i.Name
	}
}

type Message struct {
	Kind        string
	Interface   string
	Opcode      uint16
	Name        string `xml:"name,attr"`
	Description string `xml:"description"`
	Args        []Arg  `xml:"arg"`
}

type Arg struct {
	Name      string `xml:"name,attr"`
	Type      string `xml:"type,attr"`
	Interface string `xml:"interface,attr"`
}

var typemap = map[string][2]string{
	"new_id": {"proto.ObjectId", "proto.ObjectId"},
	"object": {"proto.ObjectId", "proto.ObjectId"},
	"uint":   {"proto.Uint", "uint32"},
	"string": {"proto.String", "string"},
	"int":    {"proto.Int", "int32"},
	"fd":     {"proto.Fd", "int"},
	"fixed":  {"proto.Fixed", "float32"},
	"array":  {"proto.Array", "[]byte"},
}

func Exported(parts ...string) string {
	for i := range parts {
		parts[i] = strings.Replace(strings.Title(strings.Replace(parts[i], "_", " ", -1)), " ", "", -1)
	}
	return strings.Join(parts, "")
}

var funcs = template.FuncMap{
	"Exported": Exported,
	"Unexported": func(parts ...string) string {
		name := Exported(parts...)
		name = strings.ToLower(name[0:1]) + name[1:]
		if name == "interface" {
			name += "_"
		} else if name == "error" {
			name = "err"
		}
		return name
	},
	"GoType": func(typename string) string {
		t, ok := typemap[typename]
		if !ok {
			panic(fmt.Errorf("unknown type: %s", typename))
		}
		return t[1]
	},
	"WlType": func(typename string) string {
		t, ok := typemap[typename]
		if !ok {
			panic(fmt.Errorf("unknown type: %s", typename))
		}
		return t[0]
	},
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <template> <protocol.xml> [param=value ...]", os.Args[0])
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	t := template.New("main")
	t.Funcs(funcs)

	tpl := template.Must(template.New("main").Funcs(funcs).ParseFiles(flag.Arg(0)))

	//if _, err := template.ParseFiles(flag.Arg(0)); err != nil {
	//    log.Fatal(err)
	//}

	//for _, t := range t.Templates() {
	//    t.Funcs(funcs)
	//}

	f, err := os.Open(flag.Arg(1))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer f.Close()

	var p Protocol
	d := xml.NewDecoder(f)
	if err := d.Decode(&p); err != nil {
		fmt.Println(err)
		os.Exit(3)
	}
	p.analyze()

	if err := tpl.ExecuteTemplate(os.Stdout, flag.Arg(0), p); err != nil {
		log.Fatal(err)
	}

	//go func(w io.WriteCloser) {
	//    if err := p.Go(w); err != nil {
	//        fmt.Println(err)
	//        os.Exit(4)
	//    }
	//    w.Close()
	//}(pw)

	//fset := token.NewFileSet()
	//gf, err := parser.ParseFile(fset, "messages.go", pr, 0)
	//if err != nil {
	//    panic(err)
	//}

	//if err := printer.Fprint(os.Stdout, fset, gf); err != nil {
	//    fmt.Println(err)
	//    os.Exit(5)
	//}
}
