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

type global struct {
	Name      uint32
	Interface string
	Version   uint32
}

type hello struct {
	imgPath    string
	imgW, imgH int32
	imgShm     shm.Object
	imgMap     gommap.MMap

	c *proto.Conn

	wlClient wayland.Client

	display    wayland.ClientDisplay
	registry   wayland.ClientRegistry
	shm        wayland.ClientShm
	shmPool    wayland.ClientShmPool
	compositor wayland.ClientCompositor
	surface    wayland.ClientSurface
	buffer     wayland.ClientBuffer

	xdgClient    xdg_shell.Client
	shell        xdg_shell.ClientShell
	shellSurface xdg_shell.ClientSurface

	//registryId              proto.ObjectId
	//shmId, shmPoolId        proto.ObjectId
	//compositorId, surfaceId proto.ObjectId
	//shellId, shellSurfaceId proto.ObjectId
	//bufferId                proto.ObjectId

	globals    []global
	shmFormats []uint32
}

func newHello(c *proto.Conn, imgPath string) *hello {
	h := &hello{
		imgPath:   imgPath,
		c:         c,
		wlClient:  wayland.NewClient(c),
		xdgClient: xdg_shell.NewClient(c),
	}
	h.display = h.wlClient.NewDisplay(h)
	return h
}

func (h *hello) String() string {
	b := new(bytes.Buffer)
	fmt.Fprintf(b, "Hello Wayland!\n")
	fmt.Fprintf(b, "\timage: '%s'\n", h.imgPath)
	fmt.Fprintf(b, "objects:\n")
	fmt.Fprintf(b, "\tregistry: %d\n", h.registry.Id())
	fmt.Fprintf(b, "\tshm: %d\n", h.shm.Id())
	fmt.Fprintf(b, "\tshm_pool: %d\n", h.shmPool.Id())
	fmt.Fprintf(b, "\tcompositor: %d\n", h.compositor.Id())
	fmt.Fprintf(b, "\tsurface: %d\n", h.surface.Id())
	fmt.Fprintf(b, "\tshell: %d\n", h.shell.Id())
	fmt.Fprintf(b, "\tshellSurface: %d\n", h.shellSurface.Id())
	fmt.Fprintf(b, "\tbuffer: %d\n", h.buffer.Id())

	return b.String()
}

// wayland.Display events
func (h *hello) Error(_ proto.ObjectId, _ uint32, msg string) error {
	return errgo.New("Display error: %s", msg)
}

func (h *hello) DeleteId(id uint32) error {
	h.c.DeleteObject(proto.ObjectId(id))
	return nil
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
	h.registry = h.wlClient.NewRegistry(h)
	if err := h.display.GetRegistry(h.registry.Id()); err != nil {
		return errgo.Trace(err)
	}

	if err := h.sync(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// wayland.Registry events
func (h *hello) Global(name uint32, interface_ string, version uint32) error {
	h.globals = append(h.globals, global{name, interface_, version})
	return nil
}

func (h *hello) GlobalRemove(_ uint32) error {
	return nil
}

func (h *hello) bindCompositor() error {
	for _, g := range h.globals {
		if g.Interface == "wl_compositor" {
			h.compositor = h.wlClient.NewCompositor(h)
			if err := h.registry.Bind(g.Name, g.Interface, g.Version, h.compositor.Id()); err != nil {
				return errgo.Trace(err)
			}

			return nil
		}
	}
	return errgo.New("no wl_compositor global found")
}

func (h *hello) createSurface() error {
	h.surface = h.wlClient.NewSurface(h)
	if err := h.compositor.CreateSurface(h.surface.Id()); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

// wayland.Surface events
func (h *hello) Enter(_ proto.ObjectId) error {
	return nil
}

func (h *hello) Leave(_ proto.ObjectId) error {
	return nil
}

func (h *hello) createShellSurface() error {
	// bind xdg_shell
	for _, g := range h.globals {
		if g.Interface == "xdg_shell" {
			h.shell = h.xdgClient.NewShell(h)
			if err := h.registry.Bind(g.Name, g.Interface, g.Version, h.shell.Id()); err != nil {
				return errgo.Trace(err)
			}

			goto bound
		}
	}
	return errgo.New("no xdg_shell global found")

bound:
	if err := h.shell.UseUnstableVersion(3); err != nil {
		return errgo.Trace(err)
	}

	if err := h.sync(); err != nil {
		return errgo.Trace(err)
	}

	// create shell surface
	h.shellSurface = h.xdgClient.NewSurface(h)
	if err := h.shell.GetXdgSurface(h.shellSurface.Id(), h.surface.Id()); err != nil {
		return errgo.Trace(err)
	}

	// set title
	if err := h.shellSurface.SetTitle("hello"); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// xdg_shell.Shell events
func (h *hello) Ping(serial uint32) error {
	if err := h.shell.Pong(serial); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

// xdg_shell.ShellSurface events
func (h *hello) Activated() error {
	return nil
}

func (h *hello) ChangeState(_, _, _ uint32) error {
	return nil
}

func (h *hello) Close() error {
	return nil
}

func (h *hello) Configure(_, _ int32) error {
	return nil
}

func (h *hello) Deactivated() error {
	return nil
}

func (h *hello) bindShm() error {
	for _, g := range h.globals {
		if g.Interface == "wl_shm" {
			h.shm = h.wlClient.NewShm(h)
			if err := h.registry.Bind(g.Name, g.Interface, g.Version, h.shm.Id()); err != nil {
				return errgo.Trace(err)
			}
			goto formats
		}
	}
	return errgo.New("no wl_shm global found")

formats:
	// collect shm formats
	if err := h.sync(); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

// wayland.Shm events
func (h *hello) Format(_ uint32) error {
	return nil
}

func (h *hello) createShmPool() error {
	h.shmPool = h.wlClient.NewShmPool(h)
	if err := h.shm.CreatePool(h.shmPool.Id(), h.imgShm.Fd(), int32(len(h.imgMap))); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

func (h *hello) createBuffer() error {
	h.buffer = h.wlClient.NewBuffer(h)
	if err := h.shmPool.CreateBuffer(
		h.buffer.Id(), // Id
		0,             // Offset
		h.imgW,        // Width
		h.imgH,        // Height
		h.imgW*4,      // Stride
		0,             // Format
	); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

// wayland.Buffer events
func (h *hello) Release() error {
	return nil
}

func (h *hello) attach() error {
	if err := h.surface.Attach(h.buffer.Id(), 0, 0); err != nil {
		return errgo.Trace(err)
	}
	if err := h.surface.Damage(0, 0, h.imgW, h.imgH); err != nil {
		return errgo.Trace(err)
	}
	if err := h.surface.Commit(); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

func (h *hello) loop() error {
	for {
		if err := h.c.Next(); err != nil {
			return errgo.Trace(err)
		}
	}
}

type callback struct {
	done bool
}

func (cb *callback) Done(_ uint32) error {
	cb.done = true
	return nil
}

func (h *hello) sync() error {
	cb := new(callback)
	scb := h.wlClient.NewCallback(cb)
	if err := h.display.Sync(scb.Id()); err != nil {
		return errgo.Trace(err)
	}
	for !cb.done {
		if err := h.c.Next(); err != nil {
			return errgo.Trace(err)
		}
	}
	return nil
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
