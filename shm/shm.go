package shm

// #cgo LDFLAGS: -lrt
// #include <stdlib.h>
// #include <sys/mman.h>
import "C"

import (
	"os"
	"unsafe"
)

type Object struct {
	*os.File
	name string
}

func Open(name string, flag int, perm os.FileMode) (o Object, err error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	fd, err := C.shm_open(cname, C.int(flag), C.mode_t(perm))
	if err != nil {
		return o, err
	}

	return Object{File: os.NewFile(uintptr(fd), name), name: name}, nil
}

func (o Object) Close() error {
	if err := o.File.Close(); err != nil {
		return err
	}

	cname := C.CString(o.name)
	defer C.free(unsafe.Pointer(cname))

	_, err := C.shm_unlink(cname)
	return err
}
