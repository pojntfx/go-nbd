package client

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"syscall"

	"github.com/pojntfx/tapisk/pkg/ioctl"
	"github.com/pojntfx/tapisk/pkg/protocol"
	"github.com/pojntfx/tapisk/pkg/server"
)

var (
	ErrUnsupportedNetwork   = errors.New("unsupported network")
	ErrUnknownReply         = errors.New("unknown reply")
	ErrUnknownInfo          = errors.New("unknown info")
	ErrUnknownErr           = errors.New("unknown error")
	ErrUnsupportedBlockSize = errors.New("unsupported block size")
)

type Options struct {
	ExportName string
	BlockSize  uint32
}

func negotiateNewstyle(conn net.Conn) error {
	var newstyleHeader protocol.NegotiationNewstyleHeader
	if err := binary.Read(conn, binary.BigEndian, &newstyleHeader); err != nil {
		return err
	}

	if newstyleHeader.OldstyleMagic != protocol.NEGOTIATION_MAGIC_OLDSTYLE {
		return server.ErrInvalidMagic
	}

	if newstyleHeader.OptionMagic != protocol.NEGOTIATION_MAGIC_OPTION {
		return server.ErrInvalidMagic
	}

	if _, err := conn.Write(make([]byte, 4)); err != nil { // Send client flags (uint32)
		return err
	}

	return nil
}

func Connect(conn net.Conn, device *os.File, options *Options) error {
	var cfd uintptr
	switch c := conn.(type) {
	case *net.TCPConn:
		file, err := c.File()
		if err != nil {
			return err
		}

		cfd = uintptr(file.Fd())
	case *net.UnixConn:
		file, err := c.File()
		if err != nil {
			return err
		}

		cfd = uintptr(file.Fd())
	default:
		return ErrUnsupportedNetwork
	}

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		device.Fd(),
		ioctl.NEGOTIATION_IOCTL_SET_SOCK,
		uintptr(cfd),
	); err != 0 {
		return err
	}

	if err := negotiateNewstyle(conn); err != nil {
		return err
	}

	if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
		OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
		ID:          protocol.NEGOTIATION_ID_OPTION_GO,
		Length:      0,
	}); err != nil {
		return err
	}

	exportName := []byte(options.ExportName)

	if err := binary.Write(conn, binary.BigEndian, uint32(len(exportName))); err != nil {
		return err
	}

	if _, err := conn.Write([]byte(exportName)); err != nil {
		return err
	}

	if err := binary.Write(conn, binary.BigEndian, uint16(0)); err != nil { // Send information request count (uint16)
		return err
	}

	size := uint64(0)
	chosenBlockSize := uint32(1)

n:
	for {
		var replyHeader protocol.NegotiationReplyHeader
		if err := binary.Read(conn, binary.BigEndian, &replyHeader); err != nil {
			return err
		}

		if replyHeader.ReplyMagic != protocol.NEGOTIATION_MAGIC_REPLY {
			return server.ErrInvalidMagic
		}

		switch replyHeader.Type {
		case protocol.NEGOTIATION_TYPE_REPLY_INFO:
			infoRaw := make([]byte, replyHeader.Length)
			if _, err := io.ReadFull(conn, infoRaw); err != nil {
				return err
			}

			var infoType uint16
			if err := binary.Read(bytes.NewBuffer(infoRaw), binary.BigEndian, &infoType); err != nil {
				return err
			}

			switch infoType {
			case protocol.NEGOTIATION_TYPE_INFO_EXPORT:
				var info protocol.NegotiationReplyInfo
				if err := binary.Read(bytes.NewBuffer(infoRaw), binary.BigEndian, &info); err != nil {
					return err
				}

				size = info.Size
			case protocol.NEGOTIATION_TYPE_INFO_NAME:
				// Discard export name
			case protocol.NEGOTIATION_TYPE_INFO_DESCRIPTION:
				// Discard export description
			case protocol.NEGOTIATION_TYPE_INFO_BLOCKSIZE:
				var info protocol.NegotiationReplyBlockSize
				if err := binary.Read(bytes.NewBuffer(infoRaw), binary.BigEndian, &info); err != nil {
					return err
				}

				if options.BlockSize == 0 {
					chosenBlockSize = info.PreferredBlockSize
				} else if options.BlockSize >= info.MinimumBlockSize && options.BlockSize <= info.MaximumBlockSize {
					chosenBlockSize = options.BlockSize
				} else {
					return ErrUnsupportedBlockSize
				}
			default:
				return ErrUnknownInfo
			}
		case protocol.NEGOTIATION_TYPE_REPLY_ACK:
			break n
		case protocol.NEGOTIATION_TYPE_REPLY_ERR_UNKNOWN:
			return ErrUnknownErr
		default:
			return ErrUnknownReply
		}
	}

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		device.Fd(),
		ioctl.NEGOTIATION_IOCTL_SET_SIZE_BLOCKS,
		uintptr(size/uint64(chosenBlockSize)),
	); err != 0 {
		return err
	}

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		device.Fd(),
		ioctl.NEGOTIATION_IOCTL_DO_IT,
		0,
	); err != 0 {
		return err
	}

	return nil
}

func Disconnect(device *os.File) error {
	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		device.Fd(),
		ioctl.TRANSMISSION_IOCTL_DISCONNECT,
		0,
	); err != 0 {
		return err
	}

	return nil
}

func List(conn net.Conn) ([]string, error) {
	if err := negotiateNewstyle(conn); err != nil {
		return []string{}, err
	}

	if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
		OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
		ID:          protocol.NEGOTIATION_ID_OPTION_LIST,
		Length:      0,
	}); err != nil {
		return []string{}, err
	}

	var replyHeader protocol.NegotiationReplyHeader
	if err := binary.Read(conn, binary.BigEndian, &replyHeader); err != nil {
		return []string{}, err
	}

	if replyHeader.ReplyMagic != protocol.NEGOTIATION_MAGIC_REPLY {
		return []string{}, server.ErrInvalidMagic
	}

	infoRaw := make([]byte, replyHeader.Length)
	if _, err := io.ReadFull(conn, infoRaw); err != nil {
		return []string{}, err
	}

	info := bytes.NewBuffer(infoRaw)

	exportNames := []string{}
	for {
		var exportNameLength uint32
		if err := binary.Read(info, binary.BigEndian, &exportNameLength); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return []string{}, err
		}

		exportName := make([]byte, exportNameLength)
		if _, err := io.ReadFull(info, exportName); err != nil {
			return []string{}, err
		}

		exportNames = append(exportNames, string(exportName))
	}

	if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
		OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
		ID:          protocol.NEGOTIATION_ID_OPTION_ABORT,
		Length:      0,
	}); err != nil {
		return []string{}, err
	}

	return exportNames, nil
}
