//go:build linux && cgo

package ioctl

/*
#include <sys/ioctl.h>
#include <linux/nbd.h>
*/
import "C"

const (
	TRANSMISSION_IOCTL_DISCONNECT = C.NBD_DISCONNECT
	TRANSMISSION_IOCTL_CLEAR_SOCK = C.NBD_CLEAR_SOCK
	TRANSMISSION_IOCTL_CLEAR_QUE  = C.NBD_CLEAR_QUE
)
