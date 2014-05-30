package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"io/ioutil"
	"os"
	"time"

	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/raster"

	"log"

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

type buffer struct {
	wayland.ClientBuffer
	img  image.NRGBA
	busy bool
}

func (b *buffer) Release() error {
	b.busy = false
	return nil
}

type clock struct {
	t            time.Time
	w, h, stride int32
	fn           *freetype.Context
	pt           raster.Point
	format       string

	conn *proto.Conn

	wlc wayland.Client

	display    wayland.ClientDisplay
	registry   wayland.ClientRegistry
	shm        wayland.ClientShm
	compositor wayland.ClientCompositor
	surface    wayland.ClientSurface

	xdgc         xdg_shell.Client
	shell        xdg_shell.ClientShell
	shellSurface xdg_shell.ClientSurface

	buffers [2]*buffer
	bufSize int32
	bufsMap []byte

	shmGlobal, compositorGlobal, xdgShellGlobal global
}

const PADDING = 0

func newClock(conn *proto.Conn, w, h int32, fn *freetype.Context, pt raster.Point, format string) (*clock, error) {
	c := &clock{
		conn: conn,
		w:    w, h: h,
		stride: w*4 + PADDING,
		fn:     fn, pt: pt, format: format,
	}
	c.bufSize = c.h * c.stride
	c.fn.SetClip(image.Rect(0, 0, int(w), int(h)))

	c.wlc = wayland.NewClient(conn)
	c.xdgc = xdg_shell.NewClient(conn)
	c.display = c.wlc.NewDisplay(c)

	if err := c.getRegistry(); err != nil {
		return nil, err
	}

	//if err := c.checkGlobals(); err != nil {
	//    return nil, err
	//}

	if err := c.createSurface(); err != nil {
		return nil, err
	}

	if err := c.createBuffers(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *clock) Print() {
	fmt.Printf("clock:\n")
	fmt.Printf("\twidth: %d, height: %d\n", c.w, c.h)
	fmt.Printf("\tbuffers: %d, buffer size: %d\n", len(c.buffers), c.bufSize)
	fmt.Printf("\tglobals: compositor<%d>, shm<%d>, xdg_shell<%d>\n", c.compositorGlobal.Name, c.shmGlobal.Name, c.xdgShellGlobal.Name)
}

func (c *clock) getRegistry() error {
	c.registry = c.wlc.NewRegistry(c)
	if err := c.display.GetRegistry(c.registry.Id()); err != nil {
		return errgo.Trace(err)
	}

	if err := c.sync(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

func (c *clock) createSurface() error {
	g := c.compositorGlobal
	c.compositor = c.wlc.NewCompositor(c)
	if err := c.registry.Bind(g.Name, g.Interface, g.Version, c.compositor.Id()); err != nil {
		return errgo.Trace(err)
	}

	c.surface = c.wlc.NewSurface(c)
	if err := c.compositor.CreateSurface(c.surface.Id()); err != nil {
		return errgo.Trace(err)
	}
	if err := c.surface.Damage(0, 0, c.w, c.h); err != nil {
		return errgo.Trace(err)
	}

	g = c.xdgShellGlobal
	c.shell = c.xdgc.NewShell(c)
	if err := c.registry.Bind(g.Name, g.Interface, g.Version, c.shell.Id()); err != nil {
		return errgo.Trace(err)
	}
	if err := c.shell.UseUnstableVersion(3); err != nil {
		return errgo.Trace(err)
	}

	c.shellSurface = c.xdgc.NewSurface(c)
	if err := c.shell.GetXdgSurface(c.shellSurface.Id(), c.surface.Id()); err != nil {
		return errgo.Trace(err)
	}

	if err := c.shellSurface.SetTitle("clock"); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// xdg_shell.Shell events
func (c *clock) Ping(serial uint32) error {
	if err := c.shell.Pong(serial); err != nil {
		return errgo.Trace(err)
	}
	return nil
}

// xdg_shell.ShellSurface events
func (c *clock) Activated() error {
	log.Printf("Surface activated")
	return nil
}

func (c *clock) ChangeState(state, value, serial uint32) error {
	log.Printf("Surface changed state: %d %d %d", state, value, serial)
	return nil
}

func (c *clock) Close() error {
	log.Printf("Close")
	return errgo.New("Close")
}

func (c *clock) Configure(w, h int32) error {
	log.Printf("Configure: %d %d", w, h)
	return nil
}

func (c *clock) Deactivated() error {
	log.Printf("Surface deactivated")
	return nil
}

func (c *clock) createBuffers() error {
	g := c.shmGlobal
	c.shm = c.wlc.NewShm(c)
	if err := c.registry.Bind(g.Name, g.Interface, g.Version, c.shm.Id()); err != nil {
		return errgo.Trace(err)
	}

	// collect shm formats
	if err := c.sync(); err != nil {
		return errgo.Trace(err)
	}

	// allocate pool for 2 buffers
	poolSize := int32(len(c.buffers)) * c.bufSize

	shmO, err := shm.Open("clock", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return errgo.Trace(err)
	}
	defer shmO.Close()
	if err := shmO.Truncate(int64(poolSize)); err != nil {
		return errgo.Trace(err)
	}

	if c.bufsMap, err = gommap.Map(shmO.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED); err != nil {
		return errgo.Trace(err)
	}

	shmPool := c.wlc.NewShmPool(c)
	if err := c.shm.CreatePool(shmPool.Id(), shmO.Fd(), poolSize); err != nil {
		return errgo.Trace(err)
	}

	for i := range c.buffers {
		buf := new(buffer)
		buf.img.Rect = image.Rect(0, 0, int(c.w), int(c.h))
		buf.img.Stride = int(c.w) * 4
		buf.img.Pix = c.bufsMap[i*int(c.bufSize) : (i+1)*int(c.bufSize)]

		buf.ClientBuffer = c.wlc.NewBuffer(buf)
		if err := shmPool.CreateBuffer(
			buf.Id(),           // Id
			int32(i)*c.bufSize, // Offset
			c.w,                // Width
			c.h,                // Height
			c.stride,           // Stride
			0,                  // Format
		); err != nil {
			return errgo.Trace(err)
		}
		c.buffers[i] = buf
	}

	//if err := shmPool.Destroy(); err != nil {
	//    return errgo.Trace(err)
	//}

	if err := c.sync(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// wayland.Shm events
func (c *clock) Format(_ uint32) error {
	return nil
}

type callback struct {
	done bool
}

func (cb *callback) Done(s uint32) error {
	cb.done = true
	return nil
}

func (c *clock) sync() error {
	cb := new(callback)
	scb := c.wlc.NewCallback(cb)
	if err := c.display.Sync(scb.Id()); err != nil {
		return errgo.Trace(err)
	}
	for !cb.done {
		if err := c.conn.Next(); err != nil {
			return errgo.Trace(err)
		}
	}
	return nil
}

// wayland.Display events
func (c *clock) Error(obj proto.ObjectId, _ uint32, msg string) error {
	return errgo.New("Display error: object %d: %s", obj, msg)
}

func (c *clock) DeleteId(id uint32) error {
	c.conn.DeleteObject(proto.ObjectId(id))
	return nil
}

// wayland.Registry events
func (c *clock) Global(name uint32, iface string, version uint32) error {
	switch iface {
	case "wl_compositor":
		c.compositorGlobal = global{name, iface, version}

	case "wl_shm":
		c.shmGlobal = global{name, iface, version}

	case "xdg_shell":
		c.xdgShellGlobal = global{name, iface, version}
	}

	return nil
}

func (c *clock) GlobalRemove(name uint32) error {
	return nil
}

// wayland.Surface events
func (c *clock) Enter(_ proto.ObjectId) error {
	return nil
}

func (c *clock) Leave(_ proto.ObjectId) error {
	return nil
}

func (c *clock) Tick(t time.Time) error {
	c.t = t
	fmt.Printf("tick: %s\n", c.t.Format(c.format))

	buf := c.freeBuf()
	c.paint(buf, t)
	buf.busy = true

	if err := c.surface.Attach(buf.Id(), 0, 0); err != nil {
		return errgo.Trace(err)
	}
	if err := c.surface.Damage(0, 0, c.w, c.h); err != nil {
		return errgo.Trace(err)
	}

	//frame := c.wlc.NewCallback(c)
	//if err := c.surface.Frame(frame.Id()); err != nil {
	//    return errgo.Trace(err)
	//}

	if err := c.surface.Commit(); err != nil {
		return errgo.Trace(err)
	}

	if err := c.sync(); err != nil {
		return errgo.Trace(err)
	}

	return nil
}

// frame callback
func (c *clock) Done(serial uint32) error {
	return nil
}

func (c *clock) freeBuf() *buffer {
	for _, buf := range c.buffers {
		if !buf.busy {
			return buf
		}
	}
	panic(errgo.New("no free buffer, server bug?"))
}

func abs(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func (c *clock) paint(buf *buffer, t time.Time) {
	draw.Draw(&buf.img, buf.img.Bounds(), image.White, buf.img.Bounds().Min, draw.Src)
	c.fn.SetDst(&buf.img)
	c.fn.DrawString(t.Format(c.format), c.pt)
}

var (
	size   = flag.Float64("size", 12, "font size")
	format = flag.String("format", "03:04:05", "time format; see http://golang.org/pkg/time/#Time.Format")
	width  = flag.Int("w", 100, "surface width")
	height = flag.Int("h", 40, "surface height")
)

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s <font file>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	fontFile, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprint(os.Stderr, "Error opening font file: %s", err)
		os.Exit(1)
	}
	defer fontFile.Close()

	fontData, err := ioutil.ReadAll(fontFile)
	if err != nil {
		fmt.Fprint(os.Stderr, "Error reading font: %s", err)
	}

	fn, err := freetype.ParseFont(fontData)
	if err != nil {
		fmt.Fprint(os.Stderr, "Error reading font: %s", err)
	}

	ctx := freetype.NewContext()
	ctx.SetDPI(108)
	ctx.SetFont(fn)
	ctx.SetFontSize(*size)
	ctx.SetSrc(image.Black)

	pt := freetype.Pt(4, 2+int(ctx.PointToFix32(*size)>>8))

	conn, err := proto.Dial()
	if err != nil {
		fmt.Fprint(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	c, err := newClock(conn, int32(*width), int32(*height), ctx, pt, *format)

	ticker := time.Tick(1 * time.Second)

	wlMsg := make(chan *proto.Message)
	wlErr := make(chan error, 1)
	barrier := make(chan bool)
	go func() {
		for {
			msg, err := conn.ReadMessage()
			if err != nil {
				wlErr <- err
				close(wlMsg)
				close(wlErr)
				return
			}
			wlMsg <- msg
			<-barrier
		}
	}()

	c.Print()
	c.Tick(time.Now())

mainloop:
	for {
		select {
		case t := <-ticker:
			if err = c.Tick(t); err != nil {
				break mainloop
			}

		case msg := <-wlMsg:
			if err = conn.Dispatch(msg); err != nil {
				break mainloop
			}
			barrier <- true

		case err = <-wlErr:
			break mainloop
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
