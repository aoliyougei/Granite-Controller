//go:build linux && amd64 && cgo

package native

/*
#cgo CFLAGS: -I${SRCDIR}/../../native/include
#cgo LDFLAGS: -L/opt/needle -lneedle -Wl,-rpath,/opt/needle
#include "needle.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"fmt"
	"unsafe"
)

type cgoABI struct{}

func NewABI() (ABI, error) { return &cgoABI{}, nil }

func cString(value []byte) (*C.char, func(), bool) {
	if bytes.IndexByte(value, 0) >= 0 {
		return nil, func() {}, false
	}
	pointer := C.CString(string(value))
	return pointer, func() { C.free(unsafe.Pointer(pointer)) }, true
}

func (a *cgoABI) Init(system, tools, index []byte) int {
	cs, freeS, ok := cString(system)
	if !ok {
		return -1000
	}
	defer freeS()
	ct, freeT, ok := cString(tools)
	if !ok {
		return -1000
	}
	defer freeT()
	var ci *C.char
	freeI := func() {}
	if len(index) > 0 {
		ci, freeI, ok = cString(index)
		if !ok {
			return -1000
		}
		defer freeI()
	}
	return int(C.needle_init(cs, ct, ci))
}

func (a *cgoABI) Complete(text []byte, maxNewTokens int, output []byte) int {
	if len(output) == 0 {
		return -1000
	}
	ct, freeT, ok := cString(text)
	if !ok {
		return -1000
	}
	defer freeT()
	return int(C.needle_complete(ct, C.int(maxNewTokens), (*C.char)(unsafe.Pointer(&output[0])), C.int(len(output))))
}

func (a *cgoABI) Reset() { C.needle_reset() }

func nativeSupportError() error {
	return fmt.Errorf("native Needle engine requires linux/amd64 with CGO enabled")
}
