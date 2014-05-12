// +build ignore


package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
)

var HostOrder = binary.LittleEndian

type Int int32

func (i *Int) readFrom(c *net.UnixConn) error {
	return binary.Read(c, HostOrder, i)
}

func (i Int) writeTo(c *net.UnixConn) error {
	return binary.Write(c, HostOrder, i)
}

func (i Int) size() uint16 {
	return 4
}

type Uint uint32

func (i *Uint) readFrom(c *net.UnixConn) error {
	return binary.Read(c, HostOrder, i)
}

func (i Uint) writeTo(c *net.UnixConn) error {
	return binary.Write(c, HostOrder, i)
}

func (i Uint) size() uint16 {
	return 4
}

type Fixed float32

func (f *Fixed) readFrom(c *net.UnixConn) error {
	var v uint32
	if err := binary.Read(c, HostOrder, &v); err != nil {
		return err
	}
	return nil
}

func (f Fixed) writeTo(c *net.UnixConn) error {
	var v uint32
	return binary.Write(c, HostOrder, v)
}

func (f Fixed) size() uint16 {
	return 4
}

type String string

func (s *String) readFrom(c *net.UnixConn) error {
	var sl, wl uint32
	if err := binary.Read(c, HostOrder, &sl); err != nil {
		return err
	}
	wl = sl
	if r := wl % 4; r != 0 {
		wl += 4 - r
	}
	b := make([]byte, wl)
	if _, err := c.Read(b); err != nil {
		return err
	}
	*s = String(b[:sl-1])
	return nil
}

func (s String) writeTo(c *net.UnixConn) error {
	l := s.len()
	if err := binary.Write(c, HostOrder, l); err != nil {
		return err
	}
	d := make([]byte, l)
	copy(d, s)
	_, err := c.Write(d)
	return err
}

func (s String) size() uint16 {
	return uint16(s.len()) + 4
}

// string len including null byte and padded to a 32-bit boundary
func (s String) len() uint32 {
	l := uint32(len([]byte(s))) + 1
	if r := l % 4; r != 0 {
		l += 4 - r
	}
	return l
}


func (i *ObjectId) readFrom(c *net.UnixConn) error {
	return binary.Read(c, HostOrder, i)
}

func (i ObjectId) writeTo(c *net.UnixConn) error {
	return binary.Write(c, HostOrder, i)
}

func (i ObjectId) size() uint16 {
	return 4
}

type Array []byte

func (a *Array) readFrom(c *net.UnixConn) error {
	var l uint32
	if err := binary.Read(c, HostOrder, l); err != nil {
		return err
	}
	*a = make([]byte, l)
	_, err := c.Read(*a)
	return err
}

func (a Array) writeTo(c *net.UnixConn) error {
	l := a.size()
	if err := binary.Write(c, HostOrder, l); err != nil {
		return err
	}
	d := make([]byte, l)
	copy(d, a)
	_, err := c.Write(d)
	return err
}

func (a Array) size() uint16 {
	l := uint16(len(a))
	if r := l % 4; r != 0 {
		l += 4 - r
	}
	return l
}

type Fd uintptr

func (fd *Fd) readFrom(c *net.UnixConn) error {
	var b []byte
	oob := make([]byte, 16)
	_, oobn, _, _, err := c.ReadMsgUnix(b, oob)
	if err != nil {
		return err
	}
	if oobn == 0 {
		return errors.New("error reading oob")
	}
	oob = oob[:oobn]

	scms, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return err
	}
	if len(scms) != 1 {
		return fmt.Errorf("expected 1 SocketControlMessage, got %d", len(scms))
	}
	scm := scms[0]
	fds, err := syscall.ParseUnixRights(&scm)
	if err != nil {
		return nil
	}
	if len(fds) != 1 {
		return fmt.Errorf("expected 1 fd, got %d", len(fds))
	}
	*fd = Fd(fds[0])
	return nil
}

func (fd Fd) writeTo(c *net.UnixConn) error {
	var b []byte
	oob := syscall.UnixRights(int(fd))
	_, oobn, err := c.WriteMsgUnix(b, oob, nil)
	if err != nil {
		return err
	}
	if oobn != len(oob) {
		return fmt.Errorf("expected to write %d oob bytes, wrote %d", len(oob), oobn)
	}
	return nil
}

func (fd Fd) size() uint16 {
	return 4
}
