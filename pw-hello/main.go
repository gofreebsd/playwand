package main

import (
	"image"
	"image/color"
	"log"
	"os"

	_ "image/png"

	"launchpad.net/gommap"

	"github.com/vasiliyl/playwand/proto"
	"github.com/vasiliyl/playwand/proto/wayland"
	"github.com/vasiliyl/playwand/shm"
)

type window struct {
	conn *proto.Conn
	s    wayland.ServerObjectsFactory

	d    wayland.ServerDisplay
	r    wayland.ServerRegistry
	c    wayland.ServerCompositor
	surf wayland.ServerSurface
	buf  wayland.ServerBuffer

	shm  *wlShm
	pool *shmPool

	globals map[uint32]wayland.RegistryGlobalEvent

	shmFd   uintptr
	bufSize int

	w, h int32
	done bool
}

func (w *window) DeleteId(msg wayland.DisplayDeleteIdEvent) error {
	return nil
}

func (w *window) Error(msg wayland.DisplayErrorEvent) error {
	log.Printf("Display error: %s", msg.Message)
	return nil
}

func (w *window) Global(msg wayland.RegistryGlobalEvent) error {
	w.globals[msg.Name] = msg

	switch msg.Interface {
	case "wl_compositor":
		w.c = w.s.NewCompositor(w)
		if err := w.r.Bind(msg.Name, msg.Interface, msg.Version, w.c.Id()).WriteTo(w.conn); err != nil {
			return err
		}

		// создаём сюрфейс
		w.surf = w.s.NewSurface((*surface)(nil))
		log.Printf("Surface.Id() = %d", w.surf.Id())
		if err := w.c.CreateSurface(w.surf.Id()).WriteTo(w.conn); err != nil {
			return err
		}

		// аттачим
		if err := w.attach(); err != nil {
			return err
		}

	case "wl_shm":
		// биндим shm
		w.shm = &wlShm{conn: w.conn, s: w.s, formats: make([]uint32, 0)}
		w.shm.ss = w.s.NewShm(w.shm)
		log.Printf("Shm.Id() = %d", w.shm.ss.Id())
		if err := w.r.Bind(msg.Name, msg.Interface, msg.Version, w.shm.ss.Id()).WriteTo(w.conn); err != nil {
			return err
		}

		// создаём пул
		var err error
		if w.pool, err = w.shm.createPool(w.shmFd, w.bufSize); err != nil {
			return err
		}

		// создаём буфер
		if w.buf, err = w.pool.createBuffer(w.w, w.h, w.w*4, 0); err != nil {
			return err
		}

		// аттачим
		if err := w.attach(); err != nil {
			return err
		}
	}

	return nil
}

func (w *window) GlobalRemove(msg wayland.RegistryGlobalRemoveEvent) error {
	delete(w.globals, msg.Name)
	return nil
}

func (w *window) attach() error {
	log.Printf("ATTACH!")
	return nil
}

type wlShm struct {
	conn *proto.Conn
	s    wayland.ServerObjectsFactory

	ss wayland.ServerShm

	formats []uint32
}

func (o *wlShm) Format(msg wayland.ShmFormatEvent) error {
	log.Printf("Format: 0x%08x", msg.Format)
	o.formats = append(o.formats, msg.Format)
	return nil
}

func (s *wlShm) createPool(fd uintptr, size int) (*shmPool, error) {
	p := &shmPool{conn: s.conn, s: s.s}
	p.p = s.s.NewShmPool(p)
	log.Printf("ShmPool.Id() = %d", p.p.Id())
	if err := s.ss.CreatePool(p.p.Id(), fd, int32(size)).WriteTo(s.conn); err != nil {
		return nil, err
	}
	return p, nil
}

type shmPool struct {
	conn *proto.Conn
	s    wayland.ServerObjectsFactory

	p wayland.ServerShmPool
}

func (p *shmPool) createBuffer(w, h, s int32, f uint32) (wayland.ServerBuffer, error) {
	b := p.s.NewBuffer((*buffer)(nil))
	log.Printf("Buffer.Id() = %d", b.Id())
	err := p.p.CreateBuffer(b.Id(), 0, w, h, s, f).WriteTo(p.conn)
	return b, err
}

type buffer struct{}

func (b *buffer) Release(msg wayland.BufferReleaseEvent) error {
	return nil
}

type surface struct{}

func (s *surface) Enter(msg wayland.SurfaceEnterEvent) error {
	return nil
}

func (s *surface) Leave(msg wayland.SurfaceLeaveEvent) error {
	return nil
}

func main() {
	// открываем картинку
	imgf, err := os.Open("gopher.png")
	if err != nil {
		log.Fatalf("open: %s", err)
	}
	defer imgf.Close()
	img, _, err := image.Decode(imgf)
	if err != nil {
		log.Fatalf("decode: %s", err)
	}

	imgBounds := img.Bounds()
	imgBufSize := imgBounds.Dx() * imgBounds.Dy() * 4 // argb8888

	// открываем shm
	shmO, err := shm.Open("/pw-hello", os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		log.Fatalf("shm.Open: %s", err)
	}
	defer shmO.Close()

	if err := shmO.Truncate(int64(imgBufSize)); err != nil {
		log.Fatal(err)
	}

	mmap, err := gommap.Map(shmO.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED)
	if err != nil {
		log.Fatalf("mmap: %s", err)
	}
	defer mmap.UnsafeUnmap()

	// засовываем в него картинку
	offset := 0
	for x := imgBounds.Min.X; x < imgBounds.Max.X; x++ {
		for y := imgBounds.Min.Y; y < imgBounds.Max.Y; y++ {
			oc := img.At(x, y)
			mc := color.RGBAModel.Convert(oc).(color.RGBA)
			mmap[offset], mmap[offset+1], mmap[offset+2], mmap[offset+3] = mc.A, mc.R, mc.G, mc.B
			offset += 4
		}
	}

	// звоним вяленому и создаём окно
	c, err := proto.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	w := &window{
		conn: c,
		s:    wayland.NewServer(c),

		globals: make(map[uint32]wayland.RegistryGlobalEvent),

		shmFd:   shmO.Fd(),
		bufSize: imgBufSize,

		w: int32(imgBounds.Dx()), h: int32(imgBounds.Dy()),
	}

	w.d = w.s.NewDisplay(w)
	w.r = w.s.NewRegistry(w)

	if err := w.d.GetRegistry(w.r.Id()).WriteTo(w.conn); err != nil {
		log.Fatal(err)
	}

	for !w.done {
		if err := c.Next(); err != nil {
			log.Fatal(err)
		}
	}
}
