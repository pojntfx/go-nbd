package protocol

// See https://github.com/NetworkBlockDevice/nbd/blob/master/doc/proto.md

const (
	NegotiationOldstyleMagic = uint64(0x4e42444d41474943)
	NegotiationOptionMagic   = uint64(0x49484156454F5054)

	NegotiationFlagFixedNewstyle = 1 << 0

	NegotiationOptionInfo = 6
	NegotiationOptionGo   = 7
)

type NegotiationNewstyleHeader struct {
	OldstyleMagic  uint64
	OptionMagic    uint64
	HandshakeFlags uint16
}

type NegotiationClientFlags uint32

type NegotiationOptionHeader struct {
	OptionMagic uint64
	ID          uint32
	Length      uint32
}
