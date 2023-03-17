package protocol

const (
	TRANSMISSION_MAGIC_REQUEST = uint32(0x25609513)
	TRANSMISSION_MAGIC_REPLY   = uint32(0x67446698)

	TRANSMISSION_TYPE_REQUEST_READ  = uint16(0)
	TRANSMISSION_TYPE_REQUEST_WRITE = uint16(1)
	TRANSMISSION_TYPE_REQUEST_DISC  = uint16(2)

	TRANSMISSION_ERROR_EPERM  = uint32(1)
	TRANSMISSION_ERROR_EINVAL = uint32(22)
)

type TransmissionRequestHeader struct {
	RequestMagic uint32
	CommandFlags uint16
	Type         uint16
	Handle       uint64
	Offset       uint64
	Length       uint32
}

type TransmissionReplyHeader struct {
	ReplyMagic uint32
	Error      uint32
	Handle     uint64
}
