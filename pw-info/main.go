package main

import (
	"fmt"
	"log"

	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
)

func main() {
	c, err := proto.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	did := proto.ObjectId(1)

	// display.GetRegistry
	rid := proto.ObjectId(2)
	getRegistry := &wayland.DisplayGetRegistryRequest{Registry: rid}
	mGetRegistry := proto.NewMessage(did, 1)
	if err := getRegistry.MarshalWayland(mGetRegistry); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s", mGetRegistry)
	if err := c.WriteMessage(mGetRegistry); err != nil {
		log.Fatal(err)
	}

	// display.Sync
	cbid := proto.ObjectId(3)
	sync := &wayland.DisplaySyncRequest{Callback: cbid}
	mSync := proto.NewMessage(did, 0)
	if err := sync.MarshalWayland(mSync); err != nil {
		log.Fatal(err)
	}
	if err := c.WriteMessage(mSync); err != nil {
		log.Fatal(err)
	}

	// read messages until callback's done event comes
loop:
	for {
		m, err := c.ReadMessage()
		if err != nil {
			log.Fatal(err)
		}

		switch m.Object() {
		case rid:
			// check if it is global event, unmarshal and print it
			global := new(wayland.RegistryGlobalEvent)
			if err := global.UnmarshalWayland(m); err != nil {
				log.Printf("Not a global!")
			} else {
				fmt.Printf("interface: %s, name: %d, version: %d\n", global.Interface, global.Name, global.Version)
			}

		case cbid:
			// DONE!
			break loop

		case did:
			log.Printf("Display message: %s", m)
			if m.Opcode() == 0 {
				e := new(wayland.DisplayErrorEvent)
				if err := e.UnmarshalWayland(m); err != nil {
					log.Fatal(err)
				}
				log.Printf("%+v", e)
			}
			break loop

		default:
			log.Fatal(fmt.Errorf("Unexpected message: %d:%d", m.Object(), m.Opcode()))
		}
	}

	return
}
