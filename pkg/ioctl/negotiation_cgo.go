//go:build linux && cgo

package ioctl

/*
#include <sys/ioctl.h>
#include <linux/nbd.h>
*/
import "C"

const (
	NEGOTIATION_IOCTL_SET_SOCK        = C.NBD_SET_SOCK
	NEGOTIATION_IOCTL_SET_BLOCKSIZE   = C.NBD_SET_BLKSIZE
	NEGOTIATION_IOCTL_SET_SIZE_BLOCKS = C.NBD_SET_SIZE_BLOCKS
	NEGOTIATION_IOCTL_DO_IT           = C.NBD_DO_IT
	NEGOTIATION_IOCTL_SET_TIMEOUT     = C.NBD_SET_TIMEOUT
)
