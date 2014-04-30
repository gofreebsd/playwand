package proto

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path"
)

var ByteOrder = binary.LittleEndian

type header struct {
	Object     ObjectId
	OpcodeSize uint32
}

func newHeader(o ObjectId, opcode, size uint16) header {
	return header{Object: o, OpcodeSize: (uint32(size+8) << 16) | uint32(opcode)}
}

func (h header) object() ObjectId {
	return h.Object
}

func (h header) opcode() uint16 {
	return uint16(h.OpcodeSize)
}

func (h header) size() uint16 {
	return uint16(h.OpcodeSize>>16) - 8
}

func (h header) String() string {
	return fmt.Sprintf("header{Object: %08x, Opcode: %d, Size: %d}", h.object(), h.opcode(), h.size())
}

func sockPath() string {
	rt := os.Getenv("XDG_RUNTIME_DIR")
	if rt == "" {
		rt = os.Getenv("HOME")
	}
	wd := os.Getenv("WAYLAND_DISPLAY")
	if wd == "" {
		wd = "wayland-0"
	}
	return path.Join(rt, wd)
}

type Conn struct {
	c       *net.UnixConn
	objects map[ObjectId]Object
	id      ObjectId
}

func Dial() (*Conn, error) {
	return DialPath(sockPath())
}

func DialPath(path string) (c *Conn, err error) {
	c = new(Conn)
	c.c, err = net.DialUnix("unix", nil, &net.UnixAddr{Net: "unix", Name: path})
	if err == nil {
		c.objects = make(map[ObjectId]Object)
	}
	return
}

type Listener struct {
	l *net.UnixListener
}

func Listen() (Listener, error) {
	return ListenPath(sockPath())
}

func ListenPath(path string) (l Listener, err error) {
	l.l, err = net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: path})
	return
}

func (l Listener) Accept() (c *Conn, err error) {
	c = new(Conn)
	c.c, err = l.l.AcceptUnix()
	if err == nil {
		c.objects = make(map[ObjectId]Object)
	}
	return
}

func (l Listener) Close() error {
	return l.l.Close()
}

type Type interface {
	size() uint16
	writeTo(c *net.UnixConn) error
	readFrom(c *net.UnixConn) error
}

func (c *Conn) readHeader() (h header, err error) {
	err = binary.Read(c.c, ByteOrder, &h)
	return
}

func (c *Conn) writeHeader(o ObjectId, opcode, size uint16) error {
	return binary.Write(c.c, ByteOrder, newHeader(o, opcode, size))
}

func (c *Conn) WriteValues(o ObjectId, opcode uint16, vars ...Type) error {
	var size uint16
	for _, v := range vars {
		size += v.size()
	}
	if err := c.writeHeader(o, opcode, size); err != nil {
		return err
	}
	for _, v := range vars {
		if err := v.writeTo(c.c); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) ReadValue(v Type) error {
	return v.readFrom(c.c)
}

type Object interface {
	Handle(opcode uint16, c *Conn) error
}

func (c *Conn) AddObject(id ObjectId, o Object) {
	c.objects[id] = o
}

func (c *Conn) DeleteObject(id ObjectId) {
	delete(c.objects, id)
}

func (c *Conn) NextId() ObjectId {
	c.id += 1
	return c.id
}

func (c *Conn) Next() error {
	h, err := c.readHeader()
	if err != nil {
		return err
	}

	if obj, ok := c.objects[h.object()]; ok {
		return obj.Handle(h.opcode(), c)
	}

	return fmt.Errorf("no object with id %d found", h.object())
}

func (c *Conn) Close() error {
	return c.c.Close()
}
