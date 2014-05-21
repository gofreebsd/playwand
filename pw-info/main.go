package main

import (
	"fmt"
	"log"

	"github.com/errgo/errgo"

	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
)

type global struct {
	Name      uint32
	Interface string
	Version   uint32
}

type geometry struct {
	X, Y int32
	W, H int32
}

type info struct {
	c *proto.Conn

	wlClient wayland.Client

	display  wayland.ClientDisplay
	registry wayland.ClientRegistry
	output   wayland.ClientOutput

	globals    []global
	geometries []geometry
}

func newInfo(c *proto.Conn) *info {
	i := &info{
		c:        c,
		wlClient: wayland.NewClient(c),
	}
	i.display = i.wlClient.NewDisplay(i)
	return i
}

func (i *info) Go() error {
	if err := i.getRegistry(); err != nil {
		return errgo.Trace(err)
	}

	if err := i.bindOutput(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

func (i *info) Print() {
	for _, g := range i.globals {
		fmt.Printf("interface: '%s', version: %d, name: %d\n", g.Interface, g.Version, g.Name)
		if g.Interface == "wl_output" {
			for _, g := range i.geometries {
				fmt.Printf("\tx: %d, y: %d\n\twidth: %d mm, height: %d mm\n", g.X, g.Y, g.W, g.H)
			}
		}
	}
}

// wayland.Display events
func (i *info) Error(_ proto.ObjectId, _ uint32, msg string) error {
	return errgo.New("Display error: %s", msg)
}

func (i *info) DeleteId(id uint32) error {
	i.c.DeleteObject(proto.ObjectId(id))
	return nil
}

func (i *info) getRegistry() error {
	i.registry = i.wlClient.NewRegistry(i)
	if err := i.display.GetRegistry(i.registry.Id()); err != nil {
		return errgo.Trace(err)
	}

	if err := i.sync(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// wayland.Registry events
func (i *info) Global(name uint32, interface_ string, version uint32) error {
	i.globals = append(i.globals, global{Name: name, Interface: interface_, Version: version})
	return nil
}

func (i *info) GlobalRemove(_ uint32) error {
	return nil
}

func (i *info) bindOutput() error {
	var og global
	for _, g := range i.globals {
		if g.Interface == "wl_output" {
			og = g
			goto bind
		}
	}
	return errgo.New("no output registered")

bind:
	i.output = i.wlClient.NewOutput(i)
	if err := i.registry.Bind(og.Name, og.Interface, og.Version, i.output.Id()); err != nil {
		return errgo.Trace(err)
	}

	if err := i.sync(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// wayland.Output events
func (i *info) Geometry(x, y, w, h int32, _ int32, _, _ string, _ int32) error {
	i.geometries = append(i.geometries, geometry{})
	return nil
}

func (i *info) Mode(_ uint32, _, _, _ int32) error {
	return nil
}

func (i *info) Done() error {
	return nil
}

func (i *info) Scale(_ int32) error {
	return nil
}

type callback struct {
	done bool
}

func (cb *callback) Done(_ uint32) error {
	cb.done = true
	return nil
}

func (i *info) sync() error {
	cb := new(callback)
	scb := i.wlClient.NewCallback(cb)
	log.Printf("scb.Id() = %d", scb.Id())
	if err := i.display.Sync(scb.Id()); err != nil {
		return errgo.Trace(err)
	}
	for !cb.done {
		if err := i.c.Next(); err != nil {
			log.Printf("scb.Id() = %d, err = %s", scb.Id(), err)
			return errgo.Trace(err)
		}
	}
	return nil
}

func main() {
	c, err := proto.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	i := newInfo(c)

	if err := i.Go(); err != nil {
		log.Fatalf("\n%s\n", errgo.DetailedErrorStack(err, errgo.Default))
	}

	i.Print()
}
