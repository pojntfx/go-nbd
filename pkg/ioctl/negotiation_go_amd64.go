//go:build linux && !cgo && amd64

package ioctl

// See /usr/include/linux/nbd.h

const (
	NEGOTIATION_IOCTL_SET_SOCK        = 43776
	NEGOTIATION_IOCTL_SET_BLOCKSIZE   = 43777
	NEGOTIATION_IOCTL_SET_SIZE_BLOCKS = 43783
	NEGOTIATION_IOCTL_DO_IT           = 43779
	NEGOTIATION_IOCTL_SET_TIMEOUT     = 43785
)
