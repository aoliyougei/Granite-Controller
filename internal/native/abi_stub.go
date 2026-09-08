//go:build !linux || !amd64 || !cgo

package native

import "fmt"

func NewABI() (ABI, error) {
	return nil, fmt.Errorf("native Needle engine requires linux/amd64 with CGO enabled")
}
