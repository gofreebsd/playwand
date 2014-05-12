package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path"
	"syscall"
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
	*net.UnixConn
}

func Dial() (Conn, error) {
	return DialPath(sockPath())
}

func DialPath(path string) (c Conn, err error) {
	c.UnixConn, err = net.DialUnix("unix", nil, &net.UnixAddr{Net: "unix", Name: path})
	return
}

type Listener struct {
	*net.UnixListener
}

func Listen() (Listener, error) {
	return ListenPath(sockPath())
}

func ListenPath(path string) (l Listener, err error) {
	l.UnixListener, err = net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: path})
	return
}

func (l Listener) AcceptWayland() (c Conn, err error) {
	c.UnixConn, err = l.AcceptUnix()
	return
}

func (c Conn) readHeader() (h header, err error) {
	err = binary.Read(c, ByteOrder, &h)
	return
}

func (c Conn) writeHeader(o ObjectId, opcode, size uint16) error {
	return binary.Write(c, ByteOrder, newHeader(o, opcode, size))
}

func (c Conn) ReadMessage() (m *Message, err error) {
	h, err := c.readHeader()
	if err != nil {
		return
	}
	p := make([]byte, h.size())
	oob := make([]byte, 16)
	_, oobn, _, _, err := c.ReadMsgUnix(p, oob)
	if err != nil {
		return
	}

	m = &Message{
		object: h.object(),
		opcode: h.opcode(),
		p:      bytes.NewBuffer(p),
	}

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

func (c Conn) WriteMessage(m *Message) (err error) {
	oob := syscall.UnixRights(m.fds...)
	_, _, err = c.WriteMsgUnix(m.p.Bytes(), oob, nil)
	return
}
