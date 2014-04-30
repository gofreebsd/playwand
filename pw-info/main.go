package main

import (
	"fmt"
	"os"

	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
)

type i struct {
	c           *proto.Conn
	s           wayland.ServerObjectsFactory
	d           wayland.ServerDisplay
	r           wayland.ServerRegistry
	output_done bool
	cbs         []*cb
}

func (i *i) Error(msg wayland.DisplayErrorEvent) error {
	return fmt.Errorf("Display error: %s", msg.Message)
}

func (i *i) DeleteId(msg wayland.DisplayDeleteIdEvent) error {
	return nil
}

func (i *i) Global(msg wayland.RegistryGlobalEvent) error {
	fmt.Printf("interface: %s, name: %d, version: %d\n", msg.Interface, msg.Name, msg.Version)
	if msg.Interface == "wl_output" {
		o := i.s.NewOutput(i)
		if err := i.r.Bind(msg.Name, msg.Interface, msg.Version, o.Id()).WriteTo(i.c); err != nil {
			return err
		}

	}
	return nil
}

func (i *i) GlobalRemove(msg wayland.RegistryGlobalRemoveEvent) error {
	return nil
}

func (i *i) Geometry(msg wayland.OutputGeometryEvent) error {
	fmt.Printf("Output geometry:\n")
	fmt.Printf("\tx: %d, y: %d\n", msg.X, msg.Y)
	fmt.Printf("\twidth: %d, height: %d\n", msg.PhysicalWidth, msg.PhysicalHeight)
	fmt.Printf("\tmanufacturer: %s, model: %s\n", msg.Make, msg.Model)
	return nil
}

func (i *i) Mode(msg wayland.OutputModeEvent) error {
	return nil
}

func (i *i) Scale(msg wayland.OutputScaleEvent) error {
	return nil
}

func (i *i) Done(msg wayland.OutputDoneEvent) error {
	i.output_done = true
	return nil
}

func (i *i) done() bool {
	if !i.output_done {
		return false
	}
	for _, cb := range i.cbs {
		if !*cb {
			return false
		}
	}
	return true
}

type cb bool

func (cb *cb) Done(msg wayland.CallbackDoneEvent) error {
	*cb = true
	return nil
}

func main() {
	c, err := proto.Dial()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dial: %s", err)
		os.Exit(2)
	}
	defer c.Close()

	s := wayland.NewServer(c)

	i := new(i)

	i.c = c
	i.s = s
	i.d = s.NewDisplay(i)
	i.r = s.NewRegistry(i)

	if err := i.d.GetRegistry(i.r.Id()).WriteTo(c); err != nil {
		fmt.Fprintf(os.Stderr, "GetRegistry: %s", err)
		os.Exit(1)
	}

	cb := new(cb)
	scb := s.NewCallback(cb)

	i.cbs = append(i.cbs, cb)

	if err := i.d.Sync(scb.Id()).WriteTo(c); err != nil {
		fmt.Fprintf(os.Stderr, "Sync: %s", err)
		os.Exit(1)
	}

	for !i.done() {
		if err := c.Next(); err != nil {
			fmt.Fprintf(os.Stderr, "%s", err)
			os.Exit(1)
		}
	}
}
