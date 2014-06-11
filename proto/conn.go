package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"syscall"
)

var ByteOrder = binary.LittleEndian

type Object interface {
	Handle(m *Message) error
}

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
	curid   ObjectId
}

func Dial() (*Conn, error) {
	return DialPath(sockPath())
}

func DialPath(path string) (c *Conn, err error) {
	c = new(Conn)
	c.c, err = net.DialUnix("unix", nil, &net.UnixAddr{Net: "unix", Name: path})
	if err == nil {
		c.objects = make(map[ObjectId]Object)
		c.curid = 1
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
		c.curid = 1
	}
	return
}

func (l Listener) Close() error {
	return l.l.Close()
}

func (c *Conn) readHeader() (h header, err error) {
	err = binary.Read(c.c, ByteOrder, &h)
	return
}

func (c *Conn) writeHeader(o ObjectId, opcode, size uint16) error {
	return binary.Write(c.c, ByteOrder, newHeader(o, opcode, size))
}

func (c *Conn) ReadMessage() (m *Message, err error) {
	h, err := c.readHeader()
	if err != nil {
		return
	}

	log.Printf("ReadMessage: header = %s", h)

	m = &Message{
		object: h.object(),
		opcode: h.opcode(),
	}

	if h.size() == 0 {
		return
	}

	p := make([]byte, h.size())
	oob := make([]byte, 32)
	n, oobn, _, _, err := c.c.ReadMsgUnix(p, oob)
	if err != nil {
		return
	}
	if uint16(n) != h.size() {
		err = fmt.Errorf("expected %d bytes, got %d", h.size(), n)
		return
	}

	log.Printf("ReadMessage: n = %d, oobn = %d", n, oobn)

	m.p = bytes.NewBuffer(p)

	if oobn == 0 {
		return
	}

	oob = oob[:oobn]
	scms, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return
	}
	if len(scms) != 1 {
		err = fmt.Errorf("expected 1 SocketControlMessage, got %d", len(scms))
		return
	}
	scm := scms[0]
	m.fds, err = syscall.ParseUnixRights(&scm)
	return
}

func (c *Conn) WriteMessage(m *Message) (err error) {
	var payload []byte
	if m.p != nil {
		payload = m.p.Bytes()
	}
	if err = c.writeHeader(m.object, m.opcode, uint16(len(payload))); err != nil {
		return
	}
	if len(payload) == 0 {
		// message without payload wouldn't contain fds, so we can return here
		return nil
	}
	var oob []byte
	if len(m.fds) != 0 {
		oob = syscall.UnixRights(m.fds...)
	}
	_, _, err = c.c.WriteMsgUnix(payload, oob, nil)
	return
}

func (c *Conn) AddObject(id ObjectId, o Object) {
	c.objects[id] = o
}

func (c *Conn) DeleteObject(id ObjectId) {
	delete(c.objects, id)
}

func (c *Conn) NextId() (id ObjectId) {
	id = c.curid
	c.curid++
	return
}

func (c *Conn) Next() error {
	m, err := c.ReadMessage()
	if err != nil {
		return err
	}

	return c.Dispatch(m)
}

func (c *Conn) Dispatch(m *Message) error {
	if obj, ok := c.objects[m.Object()]; ok {
		return obj.Handle(m)
	}

	return fmt.Errorf("Object %d is not registered", m.Object())
}

func (c *Conn) Close() error {
	return c.c.Close()
}
