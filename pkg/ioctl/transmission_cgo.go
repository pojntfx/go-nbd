//go:build linux && cgo

package ioctl

/*
#include <sys/ioctl.h>
#include <linux/nbd.h>
*/
import "C"

const (
	TRANSMISSION_IOCTL_DISCONNECT = C.NBD_DISCONNECT
)
