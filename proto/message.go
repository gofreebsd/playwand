package proto

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

var HostOrder = binary.LittleEndian

type ObjectId uint32

type Message struct {
	object ObjectId
	opcode uint16
	p      *bytes.Buffer
	fds    []int
	fdi    int
}

func NewMessage(object ObjectId, opcode uint16) *Message {
	return &Message{object: object, opcode: opcode, p: new(bytes.Buffer)}
}

func (m *Message) Object() ObjectId {
	return m.object
}

func (m *Message) Opcode() uint16 {
	return m.opcode
}

func (m *Message) String() string {
	return fmt.Sprintf("Message{obj:%d, opcode:%d, payload: %+v}", m.object, m.opcode, m.p.Bytes())
}

func (m *Message) ReadInt() (v int32, err error) {
	err = binary.Read(m.p, HostOrder, &v)
	return
}

func (m *Message) WriteInt(v int32) error {
	return binary.Write(m.p, HostOrder, v)
}

func (m *Message) ReadUint() (v uint32, err error) {
	err = binary.Read(m.p, HostOrder, &v)
	return
}

func (m *Message) WriteUint(v uint32) error {
	return binary.Write(m.p, HostOrder, v)
}

func (m *Message) ReadFixed() (v float32, err error) {
	var u uint32
	if err = binary.Read(m.p, HostOrder, &u); err != nil {
		return
	}
	// TODO: make float32
	return
}
func (m *Message) WriteFixed(v float32) error {
	var u uint32 // TODO: make uint32
	return binary.Write(m.p, HostOrder, u)
}

func (m *Message) ReadString() (s string, err error) {
	var l uint32
	if err = binary.Read(m.p, HostOrder, &l); err != nil {
		return
	}
	b := make([]byte, stringWireLen(l))
	if _, err = m.p.Read(b); err != nil {
		return
	}
	s = string(b[:l-1])
	return
}

func (m *Message) WriteString(s string) (err error) {
	// TODO: do we need to handle multibyte strings?
	l := uint32(len(s))
	if err = binary.Write(m.p, HostOrder, l); err != nil {
		return
	}
	d := make([]byte, stringWireLen(l))
	copy(d, s)
	_, err = m.p.Write(d)
	return
}

func stringWireLen(l uint32) uint32 {
	if r := l % 4; r != 0 {
		l += 4 - r
	}
	return l
}

func (m *Message) ReadObjectId() (oid ObjectId, err error) {
	err = binary.Read(m.p, HostOrder, &oid)
	return
}

func (m *Message) WriteObjectId(oid ObjectId) error {
	return binary.Write(m.p, HostOrder, oid)
}

func (m *Message) ReadArray() (a []byte, err error) {
	panic(fmt.Errorf("NOT IMPLEMENTED"))
	//	var l uint32
	//	if err = binary.Read(m.p, HostOrder, &l); err != nil {
	//		return err
	//	}
	//	a = make([]byte, l)
	//	_, err = m.p.Read(a)
	//	return
}

func (m *Message) WriteArray(a []byte) error {
	panic(fmt.Errorf("NOT IMPLEMENTED"))
	// l := len(a)
	// if err := binary.Write(c, HostOrder, uint32(l)); err != nil {
	// 	return err
	// }
	// d := make([]byte, l)
	// copy(d, a)
	// _, err := c.Write(d)
	// return err
}

func (m *Message) ReadFd() (fd uintptr, err error) {
	if len(m.fds) <= m.fdi {
		err = fmt.Errorf("message does not contain fd")
		return
	}
	fd = uintptr(m.fds[m.fdi])
	m.fdi++
	return
}

func (m *Message) WriteFd(fd uintptr) error {
	m.fds = append(m.fds, int(fd))
	return nil
}
