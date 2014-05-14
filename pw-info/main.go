package main

import (
	"fmt"
	"log"

	"github.com/errgo/errgo"

	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
)

type info struct {
	c                 proto.Conn
	id, did, rid, oid proto.ObjectId
	globals           []*wayland.RegistryGlobalEvent
	geometries        []*wayland.OutputGeometryEvent
}

func newInfo(c proto.Conn) *info {
	return &info{
		c:   c,
		did: 1,
		id:  2,
	}
}

func (i *info) nextId() (id proto.ObjectId) {
	id = i.id
	i.id++
	return
}

func (i *info) Go() error {
	if err := i.getRegistry(); err != nil {
		return err
	}

	if err := i.bindOutput(); err != nil {
		return err
	}

	fmt.Printf("%+v\n", i)
	return nil
}

func (i *info) Print() {
	for _, g := range i.globals {
		fmt.Printf("interface: '%s', version: %d, name: %d\n", g.Interface, g.Version, g.Name)
		if g.Interface == "wl_output" {
			for _, g := range i.geometries {
				fmt.Printf("\tx: %d, y: %d\n\twidth: %d mm, height: %d mm\n", g.X, g.Y, g.PhysicalWidth, g.PhysicalHeight)
			}
		}

	}
}

func (i *info) getRegistry() error {
	i.rid = i.nextId()
	getRegistry := &wayland.DisplayGetRegistryRequest{Registry: i.rid}
	mGetRegistry := proto.NewMessage(i.did, 1) // ☣
	if err := getRegistry.Marshal(mGetRegistry); err != nil {
		return errgo.Trace(err)
	}
	if err := i.c.WriteMessage(mGetRegistry); err != nil {
		return errgo.Trace(err)
	}

	cbid := i.nextId()
	if err := i.sync(cbid); err != nil {
		return errgo.Trace(err)
	}

cb:
	for {
		m, err := i.c.ReadMessage()
		if err != nil {
			return errgo.Trace(err)
		}

		switch {
		case m.Object() == i.did && m.Opcode() == 0:
			e := new(wayland.DisplayErrorEvent)
			if err := e.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			return errgo.New("%s", e.Message)

		case m.Object() == i.rid && m.Opcode() == 0:
			global := new(wayland.RegistryGlobalEvent)
			if err := global.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			i.globals = append(i.globals, global)

		case m.Object() == cbid:
			break cb
		}
	}

	// deleteId
	m, err := i.c.ReadMessage()
	if err != nil {
		return errgo.Trace(err)
	}

	switch {
	case m.Object() == i.did && m.Opcode() == 0:
		e := new(wayland.DisplayErrorEvent)
		if err := e.Unmarshal(m); err != nil {
			return errgo.Trace(err)
		}
		return errgo.New("%s", e.Message)

	case m.Object() == i.did && m.Opcode() == 1:
		d := new(wayland.DisplayDeleteIdEvent)
		if err := d.Unmarshal(m); err != nil {
			return errgo.Trace(err)
		}
		if proto.ObjectId(d.Id) != cbid {
			return errgo.New("unexpected message: %+v", d)
		}

	default:
		return errgo.New("unexpected message: %s", m)
	}

	return nil
}

func (i *info) bindOutput() error {
	i.oid = i.nextId()

	var og *wayland.RegistryGlobalEvent
	for _, g := range i.globals {
		if g.Interface == "wl_output" {
			og = g
			break
		}
	}
	if og == nil {
		return errgo.New("no output registered")
	}

	bind := &wayland.RegistryBindRequest{Name: og.Name, Interface: og.Interface, Version: og.Version, Id: i.oid}
	mBind := proto.NewMessage(i.rid, 0)
	if err := bind.Marshal(mBind); err != nil {
		return errgo.Trace(err)
	}
	if err := i.c.WriteMessage(mBind); err != nil {
		return errgo.Trace(err)
	}

	cbid := i.nextId()
	if err := i.sync(cbid); err != nil {
		return errgo.Trace(err)
	}

geometry:
	for {
		m, err := i.c.ReadMessage()
		if err != nil {
			return errgo.Trace(err)
		}

		switch {
		case m.Object() == i.did && m.Opcode() == 0:
			e := new(wayland.DisplayErrorEvent)
			if err := e.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			log.Printf("%+v", e)
			return errgo.New("%s", e.Message)

		case m.Object() == i.oid && m.Opcode() == 0:
			g := new(wayland.OutputGeometryEvent)
			if err := g.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			i.geometries = append(i.geometries, g)

		case m.Object() == i.oid && m.Opcode() == 3:
			break geometry

		default:
			return errgo.New("unexpected message: %s", m)
		}
	}

	return nil
}

func (i *info) sync(id proto.ObjectId) error {
	sync := &wayland.DisplaySyncRequest{Callback: id}
	mSync := proto.NewMessage(i.did, 0)
	if err := sync.Marshal(mSync); err != nil {
		return errgo.Trace(err)
	}
	return i.c.WriteMessage(mSync)
}

func main() {
	c, err := proto.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	i := newInfo(c)

	if err := i.Go(); err != nil {
		log.Fatal(errgo.DetailedErrorStack(err, errgo.Default))
	}

	i.Print()
}
