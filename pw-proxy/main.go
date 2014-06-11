package main

import (
	"log"

	"github.com/vasiliyl/playwand/proto"
)

//type display struct {
//    c *proto.ClientConn
//    s *proto.ServerConn

//    proto.ClientDisplay
//    proto.ServerDisplay

//    r *registry
//}

//func (d *display) GetRegistry(id proto.ObjectId) error {
//    log.Printf("GetRegistry: %d", id)
//    r := &registry{
//        c: d.c, s: d.s,
//        objects: make(map[proto.Uint]string),
//    }
//    r.ClientRegistry = d.c.NewRegistry(id, r)
//    r.ServerRegistry = d.s.NewRegistry(id, r)
//    return d.ServerDisplay.GetRegistry(id)
//}

//func (d *display) Sync(id proto.ObjectId) error {
//    log.Printf("Sync: %d", id)
//    cb := new(callback)
//    cb.ClientCallback = d.c.NewCallback(id, cb)
//    cb.ServerCallback = d.s.NewCallback(id, cb)
//    return d.ServerDisplay.Sync(id)
//}

//type registry struct {
//    c *proto.ClientConn
//    s *proto.ServerConn

//    proto.ClientRegistry
//    proto.ServerRegistry

//    objects map[proto.Uint]string
//}

//func (r *registry) Global(name proto.Uint, interface_ proto.String, version proto.Uint) error {
//    log.Printf("Global: name: %d, interface: %s, version: %d", name, interface_, version)
//    r.objects[name] = string(interface_)
//    return r.ClientRegistry.Global(name, interface_, version)
//}

//func (r *registry) Bind(name proto.Uint, interface_ proto.String, version proto.Uint, id proto.ObjectId) error {
//    log.Printf("Bind: %d %s %d %d", name, interface_, version, id)

//    switch r.objects[name] {
//    case "wl_shm":
//        shm := new(shm)
//        shm.ClientShm = r.c.NewShm(id, shm)
//        shm.ServerShm = r.s.NewShm(id, shm)

//    case "wl_seat":
//        seat := new(seat)
//        seat.ClientSeat = r.c.NewSeat(id, seat)
//        seat.ServerSeat = r.s.NewSeat(id, seat)

//    case "wl_output":
//        output := new(output)
//        output.ClientOutput = r.c.NewOutput(id, output)
//        output.ServerOutput = r.s.NewOutput(id, output)

//    case "wl_compositor":
//        compositor := new(compositor)
//        compositor.ClientCompositor = r.c.NewCompositor(id, compositor)
//        compositor.ServerCompositor = r.s.NewCompositor(id, compositor)

//    case "wl_subcompositor":
//        subcompositor := new(subcompositor)
//        subcompositor.ClientSubcompositor = r.c.NewSubcompositor(id, subcompositor)
//        subcompositor.ServerSubcompositor = r.s.NewSubcompositor(id, subcompositor)

//    case "wl_drm":
//        drm := new(drm)
//        drm.ClientDrm = r.c.NewDrm(id, drm)
//        drm.ServerDrm = r.s.NewDrm(id, drm)

//    default:
//        return fmt.Errorf("unsupported bind target: %s", r.objects[name])
//    }

//    return r.ServerRegistry.Bind(name, interface_, version, id)
//}

//type callback struct {
//    proto.ClientCallback
//    proto.ServerCallback
//}

//type shm struct {
//    proto.ClientShm
//    proto.ServerShm
//}

//func (s *shm) CreatePool(id proto.ObjectId, fd proto.Fd, size proto.Int) error {
//    log.Printf("Shm.CreatePool: id=%d, fd=%d, size=%d", id, fd, size)
//    return s.ServerShm.CreatePool(id, fd, size)
//}

//func (s *shm) Format(format proto.Uint) error {
//    log.Printf("Shm.Format: format=%d", format)
//    return s.ClientShm.Format(format)
//}

//type seat struct {
//    proto.ClientSeat
//    proto.ServerSeat
//}

//type output struct {
//    proto.ClientOutput
//    proto.ServerOutput
//}

//type drm struct {
//    proto.ClientDrm
//    proto.ServerDrm
//}

//func (d *drm) Authenticate(id proto.Uint) error {
//    log.Printf("Drm.Authenticate: %d", id)
//    return d.ServerDrm.Authenticate(id)
//}

//func (d *drm) CreateBuffer(id proto.ObjectId, name proto.Uint, width proto.Int, height proto.Int, stride proto.Uint, format proto.Uint) error {
//    log.Printf("Drm.CreateBuffer: %d", id)
//    return d.ServerDrm.CreateBuffer(id, name, width, height, stride, format)
//}

//func (d *drm) CreatePlanarBuffer(id proto.ObjectId, name proto.Uint, width proto.Int, height proto.Int, format proto.Uint, offset0 proto.Int, stride0 proto.Int, offset1 proto.Int, stride1 proto.Int, offset2 proto.Int, stride2 proto.Int) error {

//}

//type compositor struct {
//    proto.ClientCompositor
//    proto.ServerCompositor
//}

//type subcompositor struct {
//    proto.ClientSubcompositor
//    proto.ServerSubcompositor
//}

func proxy(src, dst *proto.Conn, name string) {
	for {
		msg, err := src.ReadMessage()
		if err != nil {
			log.Printf("%s: ReadMessage: %s", name, err)
			break
		}

		log.Printf("%s: %s", name, msg)

		if err := dst.WriteMessage(msg); err != nil {
			log.Printf("%s: WriteMessage: %s", name, err)
			break
		}
	}
	src.Close()
	dst.Close()
}

func main() {
	log.SetFlags(log.Llongfile)
	l, err := proto.ListenPath("/run/user/1000/wayland-1")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			log.Print(err)
			break
		}

		s, err := proto.Dial()
		if err != nil {
			log.Print(err)
			c.Close()
			break
		}

		go proxy(c, s, "c->s")
		go proxy(s, c, "s->c")
	}
}
