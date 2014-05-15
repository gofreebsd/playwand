package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"

	_ "image/jpeg"
	_ "image/png"

	"launchpad.net/gommap"

	"github.com/errgo/errgo"
	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
	"github.com/vasiliyl/playwand/proto/xdg_shell"
	"github.com/vasiliyl/playwand/shm"
)

type hello struct {
	imgPath    string
	imgW, imgH int32
	imgShm     shm.Object
	imgMap     gommap.MMap

	c       proto.Conn
	id, did proto.ObjectId

	registryId              proto.ObjectId
	shmId, shmPoolId        proto.ObjectId
	compositorId, surfaceId proto.ObjectId
	shellId, shellSurfaceId proto.ObjectId
	bufferId                proto.ObjectId

	globals    []*wayland.RegistryGlobalEvent
	shmFormats []uint32
}

func newHello(c proto.Conn, imgPath string) *hello {
	return &hello{
		imgPath: imgPath,
		c:       c,
		did:     1,
		id:      2,
	}
}

func (h *hello) nextId() (id proto.ObjectId) {
	id = h.id
	h.id++
	return
}

func (h *hello) String() string {
	b := new(bytes.Buffer)
	fmt.Fprintf(b, "Hello Wayland!\n")
	fmt.Fprintf(b, "\timage: '%s'\n", h.imgPath)
	fmt.Fprintf(b, "objects:\n")
	fmt.Fprintf(b, "\tregistry: %d\n", h.registryId)
	fmt.Fprintf(b, "\tshm: %d\n", h.shmId)
	fmt.Fprintf(b, "\tshm_pool: %d\n", h.shmPoolId)
	fmt.Fprintf(b, "\tcompositor: %d\n", h.compositorId)
	fmt.Fprintf(b, "\tsurface: %d\n", h.surfaceId)
	fmt.Fprintf(b, "\tshell: %d\n", h.shellId)
	fmt.Fprintf(b, "\tshellSurface: %d\n", h.shellSurfaceId)
	fmt.Fprintf(b, "\tbuffer: %d\n", h.bufferId)

	return b.String()
}

func (h *hello) Go() error {
	if err := h.loadImage(); err != nil {
		return errgo.Trace(err)
	}

	if err := h.getRegistry(); err != nil {
		return errgo.Trace(err)
	}
	if err := h.bindCompositor(); err != nil {
		return errgo.Trace(err)
	}
	if err := h.createSurface(); err != nil {
		return errgo.Trace(err)
	}

	if err := h.createShellSurface(); err != nil {
		return errgo.Trace(err)
	}

	if err := h.bindShm(); err != nil {
		return errgo.Trace(err)
	}
	if err := h.createShmPool(); err != nil {
		return errgo.Trace(err)
	}
	if err := h.createBuffer(); err != nil {
		return errgo.Trace(err)
	}

	if err := h.attach(); err != nil {
		return errgo.Trace(err)
	}

	return h.loop()
}

func (h *hello) loadImage() error {
	// открываем картинку
	imgf, err := os.Open(h.imgPath)
	if err != nil {
		return errgo.Trace(err)
	}
	defer imgf.Close()

	img, _, err := image.Decode(imgf)
	if err != nil {
		errgo.Trace(err)
	}

	imgBounds := img.Bounds()
	imgBufSize := imgBounds.Dx() * imgBounds.Dy() * 4 // argb8888
	h.imgW, h.imgH = int32(imgBounds.Dx()), int32(imgBounds.Dy())

	// открываем shm
	if h.imgShm, err = shm.Open("/pw-hello", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm); err != nil {
		errgo.Trace(err)
	}
	if err := h.imgShm.Truncate(int64(imgBufSize)); err != nil {
		return errgo.Trace(err)
	}
	if h.imgMap, err = gommap.Map(h.imgShm.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED); err != nil {
		errgo.Trace(err)
	}

	// засовываем в него картинку
	offset := 0
	for y := imgBounds.Min.Y; y < imgBounds.Max.Y; y++ {
		for x := imgBounds.Min.X; x < imgBounds.Max.X; x++ {
			oc := img.At(x, y)
			mc := color.RGBAModel.Convert(oc).(color.RGBA)
			h.imgMap[offset], h.imgMap[offset+1], h.imgMap[offset+2], h.imgMap[offset+3] = mc.B, mc.G, mc.R, mc.A
			offset += 4
		}
	}

	return nil
}

func (h *hello) getRegistry() error {
	h.registryId = h.nextId()
	getRegistry := &wayland.DisplayGetRegistryRequest{Registry: h.registryId}
	mGetRegistry := proto.NewMessage(h.did, 1)
	if err := getRegistry.Marshal(mGetRegistry); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(mGetRegistry); err != nil {
		return errgo.Trace(err)
	}

	cbid := h.nextId()
	if err := h.sync(cbid); err != nil {
		return errgo.Trace(err)
	}

cb:
	for {
		m, err := h.c.ReadMessage()
		if err != nil {
			return errgo.Trace(err)
		}

		switch {
		case m.Object() == h.did && m.Opcode() == 0:
			e := new(wayland.DisplayErrorEvent)
			if err := e.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			return errgo.New("%s", e.Message)

		case m.Object() == h.registryId && m.Opcode() == 0:
			global := new(wayland.RegistryGlobalEvent)
			if err := global.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			h.globals = append(h.globals, global)

		case m.Object() == cbid:
			break cb
		}
	}

	// deleteId
	m, err := h.c.ReadMessage()
	if err != nil {
		return errgo.Trace(err)
	}

	switch {
	case m.Object() == h.did && m.Opcode() == 0:
		e := new(wayland.DisplayErrorEvent)
		if err := e.Unmarshal(m); err != nil {
			return errgo.Trace(err)
		}
		return errgo.New("%s", e.Message)

	case m.Object() == h.did && m.Opcode() == 1:
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

func (h *hello) bindCompositor() error {
	for _, g := range h.globals {
		if g.Interface == "wl_compositor" {
			h.compositorId = h.nextId()
			bind := &wayland.RegistryBindRequest{Name: g.Name, Interface: g.Interface, Version: g.Version, Id: h.compositorId}
			mBind := proto.NewMessage(h.registryId, 0)
			if err := bind.Marshal(mBind); err != nil {
				return errgo.Trace(err)
			}
			if err := h.c.WriteMessage(mBind); err != nil {
				return errgo.Trace(err)
			}

			return nil
		}
	}
	return errgo.New("no wl_compositor global found")
}

func (h *hello) createSurface() error {
	h.surfaceId = h.nextId()
	createSurface := &wayland.CompositorCreateSurfaceRequest{Id: h.surfaceId}
	m := proto.NewMessage(h.compositorId, 0)
	if err := createSurface.Marshal(m); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(m); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

func (h *hello) createShellSurface() error {
	for _, g := range h.globals {
		if g.Interface == "xdg_shell" {
			h.shellId = h.nextId()
			bind := &wayland.RegistryBindRequest{Name: g.Name, Interface: g.Interface, Version: g.Version, Id: h.shellId}
			m := proto.NewMessage(h.registryId, 0)
			if err := bind.Marshal(m); err != nil {
				return errgo.Trace(err)
			}
			if err := h.c.WriteMessage(m); err != nil {
				return errgo.Trace(err)
			}

			goto create_shell
		}
	}
	return errgo.New("no xdg_shell global found")

create_shell:
	// use unstable version
	useUnstableVersion := &xdg_shell.XdgShellUseUnstableVersionRequest{Version: 3}
	m := proto.NewMessage(h.shellId, 0)
	if err := useUnstableVersion.Marshal(m); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(m); err != nil {
		return errgo.Trace(err)
	}

	// sync
	cbid := h.nextId()
	if err := h.sync(cbid); err != nil {
		return errgo.Trace(err)
	}

	m, err := h.c.ReadMessage()
	if err != nil {
		return errgo.Trace(err)
	}

	switch {
	case m.Object() == h.did && m.Opcode() == 0:
		e := new(wayland.DisplayErrorEvent)
		if err := e.Unmarshal(m); err != nil {
			return errgo.Trace(err)
		}
		return errgo.New("%s", e.Message)

	case m.Object() == cbid:
		break

	default:
		return errgo.New("unexpected message: %s", m)
	}

	// deleteId for cbid
	m, err = h.c.ReadMessage()
	if err != nil {
		return errgo.Trace(err)
	}

	switch {
	case m.Object() == h.did && m.Opcode() == 0:
		e := new(wayland.DisplayErrorEvent)
		if err := e.Unmarshal(m); err != nil {
			return errgo.Trace(err)
		}
		return errgo.New("%s", e.Message)

	case m.Object() == h.did && m.Opcode() == 1:
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

	h.shellSurfaceId = h.nextId()
	createShellSurface := &xdg_shell.XdgShellGetXdgSurfaceRequest{h.shellSurfaceId, h.surfaceId}
	m = proto.NewMessage(h.shellId, 1)
	if err := createShellSurface.Marshal(m); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(m); err != nil {
		return errgo.Trace(err)
	}

	setTitle := &xdg_shell.XdgSurfaceSetTitleRequest{Title: "hello"}
	m = proto.NewMessage(h.shellSurfaceId, 3)
	if err := setTitle.Marshal(m); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(m); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

func (h *hello) bindShm() error {
	for _, g := range h.globals {
		if g.Interface == "wl_shm" {
			h.shmId = h.nextId()
			bind := &wayland.RegistryBindRequest{Name: g.Name, Interface: g.Interface, Version: g.Version, Id: h.shmId}
			mBind := proto.NewMessage(h.registryId, 0)
			if err := bind.Marshal(mBind); err != nil {
				return errgo.Trace(err)
			}
			if err := h.c.WriteMessage(mBind); err != nil {
				return errgo.Trace(err)
			}

			goto formats
		}
	}
	return errgo.New("no wl_shm global found")

formats:
	// sync and collect shm formats
	cbid := h.nextId()
	if err := h.sync(cbid); err != nil {
		return errgo.Trace(err)
	}

formats_loop:
	for {
		m, err := h.c.ReadMessage()
		if err != nil {
			return errgo.Trace(err)
		}

		switch {
		case m.Object() == h.did && m.Opcode() == 0:
			e := new(wayland.DisplayErrorEvent)
			if err := e.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			return errgo.New("%s", e.Message)

		case m.Object() == h.shmId && m.Opcode() == 0:
			f := new(wayland.ShmFormatEvent)
			if err := f.Unmarshal(m); err != nil {
				return errgo.Trace(err)
			}
			h.shmFormats = append(h.shmFormats, f.Format)

		case m.Object() == cbid:
			break formats_loop

		default:
			return errgo.New("unexpected message: %s", m)
		}
	}

	// deleteId for cbid
	m, err := h.c.ReadMessage()
	if err != nil {
		return errgo.Trace(err)
	}

	switch {
	case m.Object() == h.did && m.Opcode() == 0:
		e := new(wayland.DisplayErrorEvent)
		if err := e.Unmarshal(m); err != nil {
			return errgo.Trace(err)
		}
		return errgo.New("%s", e.Message)

	case m.Object() == h.did && m.Opcode() == 1:
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

func (h *hello) createShmPool() error {
	h.shmPoolId = h.nextId()
	createPool := &wayland.ShmCreatePoolRequest{Id: h.shmPoolId, Fd: h.imgShm.Fd(), Size: int32(len(h.imgMap))}
	m := proto.NewMessage(h.shmId, 0)
	if err := createPool.Marshal(m); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(m); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

func (h *hello) createBuffer() error {
	h.bufferId = h.nextId()
	createBuffer := &wayland.ShmPoolCreateBufferRequest{
		Id:     h.bufferId,
		Offset: 0,
		Width:  h.imgW,
		Height: h.imgH,
		Stride: h.imgW * 4,
		Format: 0,
	}
	m := proto.NewMessage(h.shmPoolId, 0)
	if err := createBuffer.Marshal(m); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(m); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

func (h *hello) attach() error {
	attach := &wayland.SurfaceAttachRequest{
		Buffer: h.bufferId,
		X:      0,
		Y:      0,
	}
	ma := proto.NewMessage(h.surfaceId, 1)
	if err := attach.Marshal(ma); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(ma); err != nil {
		return errgo.Trace(err)
	}

	dmg := &wayland.SurfaceDamageRequest{
		X: 0, Y: 0,
		Width: h.imgW, Height: h.imgH,
	}
	md := proto.NewMessage(h.surfaceId, 2)
	if err := dmg.Marshal(md); err != nil {
		return errgo.Trace(err)
	}
	if err := h.c.WriteMessage(md); err != nil {
		return errgo.Trace(err)
	}

	mcommit := proto.NewMessage(h.surfaceId, 6)
	if err := h.c.WriteMessage(mcommit); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

func (h *hello) loop() error {
	for {
		m, err := h.c.ReadMessage()
		if err != nil {
			return errgo.Trace(err)
		}

		switch {
		case m.Object() == h.did:
			if m.Opcode() == 0 {
				e := new(wayland.DisplayErrorEvent)
				if err := e.Unmarshal(m); err != nil {
					return errgo.Trace(err)
				}
				return errgo.New("Display Error: %s", e.Message)
			}

		default:
			log.Println(m)
		}
	}
}

func (h *hello) sync(id proto.ObjectId) error {
	sync := &wayland.DisplaySyncRequest{Callback: id}
	mSync := proto.NewMessage(h.did, 0)
	if err := sync.Marshal(mSync); err != nil {
		return errgo.Trace(err)
	}
	return h.c.WriteMessage(mSync)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <image>\n", os.Args[0])
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	// звоним вяленому и создаём окно
	c, err := proto.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	h := newHello(c, flag.Arg(0))
	if err := h.Go(); err != nil {
		log.Fatal(errgo.DetailedErrorStack(err, errgo.Default))
	}
}
