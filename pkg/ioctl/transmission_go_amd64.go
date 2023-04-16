//go:build linux && !cgo && amd64

package ioctl

const (
	TRANSMISSION_IOCTL_DISCONNECT = 43784
	TRAMSMISSION_IOCTL_CLEAR_SOCK = 43780
)
