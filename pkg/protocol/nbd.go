package protocol

// Port of https://github.com/freqlabs/nbd-client/blob/master/nbd-protocol.h#L40
// For more info, see https://github.com/NetworkBlockDevice/nbd/blob/master/doc/proto.md

/**
 ** Network protocol header structs and values
 **/

const (
	NbdDefaultPort = 10809
)

/*
 * Negotiation handshake
 *
 * Oldstyle:
 * When the client connects, the server sends a handshake packet with the
 * export size and supported flags.  There is no option negotiation, the
 * server immedately transitions to the transmission phase.  The oldstyle
 * protocol is deprecated and unsupported.
 *
 * When the client connects, the server sends a handshake packet with the
 * handshake flags.  The client responds with its client flags, which must
 * include the FIXED_NEWSTYLE flag.
 */

const (
	NbdMagic         = uint64(0x4e42444d41474943)
	NbdOldstyleMagic = uint64(0x00420281861253)
)

type NbdOldstyleNegotiation struct {
	Magic         uint64
	OldstyleMagic uint64
	Size          uint64
	Flags         uint32
	Reserved      [124]uint8
}

const (
	NbdNewstyleMagic = uint64(0x49484156454F5054)

	NbdFlagFixedNewstyle = (1 << 0)
	NbdFlagNoZeroes      = (1 << 1)
)

type NbdNewstyleNegotiation struct {
	Magic          uint64
	NewstyleMagic  uint64
	HandshakeFlags uint16
}

const (
	NbdClientFlagFixedNewstyle = NbdFlagFixedNewstyle
	NbdClientFlagNoZeroes      = NbdFlagNoZeroes
)

type NbdClientFlags struct {
	ClientFlags uint32
}

/*
 * Option haggling
 *
 * After the initial handshake, the client requests desired options and the
 * server replies to each option acknowledging if supported or with an
 * error if the option is unsupported.
 *
 * The client must only negotiate one option at a time.  Some options will
 * have multiple replies from the server.  The client must wait until the
 * final reply for an option is received before moving on.
 *
 * Option haggling is completed when the client sends either of the
 * following options:
 *  - EXPORT_NAME (transition to transmission mode)
 *  - ABORT (soft disconnect, server should acknowledge)
 * Alternatively, a hard disconnect may occur by disconnecting the TCP
 * session.
 *
 * The server's reply to the EXPORT_NAME option is unique.  EXPORT_NAME
 * signals a transition into transmission mode, and the server sends the
 * length of the export in bytes, the transmission flags, and unless the
 * NO_ZEROES flag has been negotiated during the handshake, 124 zero bytes
 * (reserved for future use).  If the server instead refuses the requested
 * export, it terminates closes the TCP session.
 *
 * Note: The transmission flags for the server's reply to EXPORT_NAME are
 * defined in the next section.
 */

const (
	NbdOptionMagic = NbdNewstyleMagic
)

const (
	NbdOptionExportName      = 1
	NbdOptionAbort           = 2
	NbdOptionList            = 3
	NbdOptionPeekExport      = 4 // withdrawn
	NbdOptionSTARTTLS        = 5
	NbdOptionInfo            = 6 // experimental extension
	NbdOptionGo              = 7 // experimental extension
	NbdOptionStructuredReply = 8 // experimental extension
	NbdOptionBlockSize       = 9 // experimental extension
)

type NbdOption struct {
	Magic  uint64
	Option uint32
	Length uint32
	// Data   []byte // Sent separately
}

const (
	NbdOptionReplyMagic = uint64(0x3e889045565a9)
)

const (
	NbdReplyAck    = 1
	NbdReplyServer = 2
	NbdReplyInfo   = 3 // experimental extension

	NbdReplyError              = (1 << 31)
	NbdReplyErrorUnsupported   = (1 | NbdReplyError)
	NbdReplyErrorPolicy        = (2 | NbdReplyError)
	NbdReplyErrorInvalid       = (3 | NbdReplyError)
	NbdReplyErrorPlatform      = (4 | NbdReplyError) // unused
	NbdReplyErrorTLSRequired   = (5 | NbdReplyError)
	NbdReplyErrorUnknown       = (6 | NbdReplyError) // experimental extension
	NbdReplyErrorShutdown      = (7 | NbdReplyError)
	NbdReplyErrorBlockSizeREQD = (8 | NbdReplyError) // experimental extension
)

type NbdOptionReply struct {
	Magic  uint64
	Option uint32
	Typ    int32
	Length uint32
	// Data   []byte // Sent separately
}

type NbdOptionReplyServer struct {
	Length     uint32
	ExportName string
}

/* See the next section for the definitions of the transmission flags. */

type NbdExportInfo struct {
	Size              uint64
	TransmissionFlags uint16
	Reserved          [124]uint8
}

/*
 * Transmission
 *
 * The client sends a request, and the server replies.  Replies may not
 * necessarily be in the same order as the requests, so the client assigns
 * a handle to each request.  The handle must be unique among all active
 * requests.  The server replies using the same handle to associate the
 * reply with the correct transaction.
 *
 * The following ordering constraints apply to transmissions:
 *  - All write commands must be completed before a flush command can be
 *    processed.
 *  - Data sent by the client with the FUA flag set must be written to
 *    persistent storage by the server before the server may reply.
 *
 * Only the client may cleanly disconnect during transmission, by sending
 * the DISCONNECT command.  Either the client or server may perform a hard
 * disconnect by dropping the TCP session.  If a client receives ESHUTDOWN
 * errors it must attempt a clean disconnect.
 *
 * Note on errors: The server should map EDQUOT and EFBIG to ENOSPC.
 */

const (
	NbdRequestMagic = uint32(0x25609513)

	NbdFlagHasFlags   = (1 << 0)
	NbdFlagReadOnly   = (1 << 1)
	NbdFlagSendFlush  = (1 << 2)
	NbdFlagSendFUA    = (1 << 3) /* FUA = force unit access */
	NbdFlagRotational = (1 << 4) /* use elevator algorithm */
	NbdFlagSendTrim   = (1 << 5)
)

const (
	NbdCmdRead       = 0
	NbdCmdWrite      = 1
	NbdCmdDisconnect = 2
	NbdCmdFlush      = 3
	NbdCmdTrim       = 4
)

type NbdRequest struct {
	Magic   uint32
	Flags   uint16
	Command uint16
	Handle  uint64
	Offset  uint64
	Length  uint32
	// Data    []byte // Sent separately
}

const (
	NbdReplyMagic = uint32(0x67446698)
)

const (
	NbdEPERM     = 1
	NbdEIO       = 5
	NbdENOMEM    = 12
	NbdEINVAL    = 22
	NbdENOSPC    = 28
	NbdEOVERFLOW = 75 // (experimental extension)
	NbdESHUTDOWN = 108
)

type NbdReply struct {
	Magic  uint32
	Err    uint32
	Handle uint64
	// Data   []byte // Sent separately
}
