package main

import (
	"fmt"
	"log"

	"github.com/errgo/errgo"

	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
)

type info struct {
	c *proto.Conn

	wlClient wayland.Client

	display  wayland.ClientDisplay
	registry wayland.ClientRegistry
	output   wayland.ClientOutput

	globals    []wayland.RegistryGlobalEvent
	geometries []wayland.OutputGeometryEvent
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
				fmt.Printf("\tx: %d, y: %d\n\twidth: %d mm, height: %d mm\n", g.X, g.Y, g.PhysicalWidth, g.PhysicalHeight)
			}
		}
	}
}

// wayland.Display events
func (i *info) Error(m wayland.DisplayErrorEvent) error {
	return errgo.New("Display error: %s", m.Message)
}

func (i *info) DeleteId(m wayland.DisplayDeleteIdEvent) error {
	i.c.DeleteObject(proto.ObjectId(m.Id))
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
func (i *info) Global(g wayland.RegistryGlobalEvent) error {
	i.globals = append(i.globals, g)
	return nil
}

func (i *info) GlobalRemove(_ wayland.RegistryGlobalRemoveEvent) error {
	return nil
}

func (i *info) bindOutput() error {
	var og wayland.RegistryGlobalEvent
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
func (i *info) Geometry(g wayland.OutputGeometryEvent) error {
	i.geometries = append(i.geometries, g)
	return nil
}

func (i *info) Mode(_ wayland.OutputModeEvent) error {
	return nil
}

func (i *info) Done(_ wayland.OutputDoneEvent) error {
	return nil
}

func (i *info) Scale(_ wayland.OutputScaleEvent) error {
	return nil
}

type callback struct {
	done bool
}

func (cb *callback) Done(_ wayland.CallbackDoneEvent) error {
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
