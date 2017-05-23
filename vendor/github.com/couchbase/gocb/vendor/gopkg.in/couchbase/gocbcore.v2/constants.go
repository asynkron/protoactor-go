package gocbcore

const (
	GoCbCoreVersionStr = "v1.1.2"
)

type CommandMagic uint8

const (
	ReqMagic = CommandMagic(0x80)
	ResMagic = CommandMagic(0x81)
)

// CommandCode for memcached packets.
type CommandCode uint8

const (
	CmdGet                  = CommandCode(0x00)
	CmdSet                  = CommandCode(0x01)
	CmdAdd                  = CommandCode(0x02)
	CmdReplace              = CommandCode(0x03)
	CmdDelete               = CommandCode(0x04)
	CmdIncrement            = CommandCode(0x05)
	CmdDecrement            = CommandCode(0x06)
	CmdAppend               = CommandCode(0x0e)
	CmdPrepend              = CommandCode(0x0f)
	CmdStat                 = CommandCode(0x10)
	CmdTouch                = CommandCode(0x1c)
	CmdGAT                  = CommandCode(0x1d)
	CmdHello                = CommandCode(0x1f)
	CmdSASLListMechs        = CommandCode(0x20)
	CmdSASLAuth             = CommandCode(0x21)
	CmdSASLStep             = CommandCode(0x22)
	CmdGetAllVBSeqnos       = CommandCode(0x48)
	CmdDcpOpenConnection    = CommandCode(0x50)
	CmdDcpAddStream         = CommandCode(0x51)
	CmdDcpCloseStream       = CommandCode(0x52)
	CmdDcpStreamReq         = CommandCode(0x53)
	CmdDcpGetFailoverLog    = CommandCode(0x54)
	CmdDcpStreamEnd         = CommandCode(0x55)
	CmdDcpSnapshotMarker    = CommandCode(0x56)
	CmdDcpMutation          = CommandCode(0x57)
	CmdDcpDeletion          = CommandCode(0x58)
	CmdDcpExpiration        = CommandCode(0x59)
	CmdDcpFlush             = CommandCode(0x5a)
	CmdDcpSetVbucketState   = CommandCode(0x5b)
	CmdDcpNoop              = CommandCode(0x5c)
	CmdDcpBufferAck         = CommandCode(0x5d)
	CmdDcpControl           = CommandCode(0x5e)
	CmdGetReplica           = CommandCode(0x83)
	CmdSelectBucket         = CommandCode(0x89)
	CmdObserveSeqNo         = CommandCode(0x91)
	CmdObserve              = CommandCode(0x92)
	CmdGetLocked            = CommandCode(0x94)
	CmdUnlockKey            = CommandCode(0x95)
	CmdSetMeta              = CommandCode(0xa2)
	CmdDelMeta              = CommandCode(0xa8)
	CmdGetClusterConfig     = CommandCode(0xb5)
	CmdGetRandom            = CommandCode(0xb6)
	CmdSubDocGet            = CommandCode(0xc5)
	CmdSubDocExists         = CommandCode(0xc6)
	CmdSubDocDictAdd        = CommandCode(0xc7)
	CmdSubDocDictSet        = CommandCode(0xc8)
	CmdSubDocDelete         = CommandCode(0xc9)
	CmdSubDocReplace        = CommandCode(0xca)
	CmdSubDocArrayPushLast  = CommandCode(0xcb)
	CmdSubDocArrayPushFirst = CommandCode(0xcc)
	CmdSubDocArrayInsert    = CommandCode(0xcd)
	CmdSubDocArrayAddUnique = CommandCode(0xce)
	CmdSubDocCounter        = CommandCode(0xcf)
	CmdSubDocMultiLookup    = CommandCode(0xd0)
	CmdSubDocMultiMutation  = CommandCode(0xd1)
)

type SubDocFlag uint16

const (
	SubDocFlagMkDirP = SubDocFlag(0x01)
)

type SubDocOpType uint8

const (
	SubDocOpGet            = SubDocOpType(CmdSubDocGet)
	SubDocOpExists         = SubDocOpType(CmdSubDocExists)
	SubDocOpDictAdd        = SubDocOpType(CmdSubDocDictAdd)
	SubDocOpDictSet        = SubDocOpType(CmdSubDocDictSet)
	SubDocOpDelete         = SubDocOpType(CmdSubDocDelete)
	SubDocOpReplace        = SubDocOpType(CmdSubDocReplace)
	SubDocOpArrayPushLast  = SubDocOpType(CmdSubDocArrayPushLast)
	SubDocOpArrayPushFirst = SubDocOpType(CmdSubDocArrayPushFirst)
	SubDocOpArrayInsert    = SubDocOpType(CmdSubDocArrayInsert)
	SubDocOpArrayAddUnique = SubDocOpType(CmdSubDocArrayAddUnique)
	SubDocOpCounter        = SubDocOpType(CmdSubDocCounter)
)

type HelloFeature uint16

const (
	FeatureDatatype = HelloFeature(0x01)
	FeatureSeqNo    = HelloFeature(0x04)
)

// Status field for memcached response.
type StatusCode uint16

const (
	StatusSuccess            = StatusCode(0x00)
	StatusKeyNotFound        = StatusCode(0x01)
	StatusKeyExists          = StatusCode(0x02)
	StatusTooBig             = StatusCode(0x03)
	StatusInvalidArgs        = StatusCode(0x04)
	StatusNotStored          = StatusCode(0x05)
	StatusBadDelta           = StatusCode(0x06)
	StatusNotMyVBucket       = StatusCode(0x07)
	StatusNoBucket           = StatusCode(0x08)
	StatusAuthStale          = StatusCode(0x1f)
	StatusAuthError          = StatusCode(0x20)
	StatusAuthContinue       = StatusCode(0x21)
	StatusRangeError         = StatusCode(0x22)
	StatusRollback           = StatusCode(0x23)
	StatusAccessError        = StatusCode(0x24)
	StatusNotInitialized     = StatusCode(0x25)
	StatusUnknownCommand     = StatusCode(0x81)
	StatusOutOfMemory        = StatusCode(0x82)
	StatusNotSupported       = StatusCode(0x83)
	StatusInternalError      = StatusCode(0x84)
	StatusBusy               = StatusCode(0x85)
	StatusTmpFail            = StatusCode(0x86)
	StatusSubDocPathNotFound = StatusCode(0xc0)
	StatusSubDocPathMismatch = StatusCode(0xc1)
	StatusSubDocPathInvalid  = StatusCode(0xc2)
	StatusSubDocPathTooBig   = StatusCode(0xc3)
	StatusSubDocDocTooDeep   = StatusCode(0xc4)
	StatusSubDocCantInsert   = StatusCode(0xc5)
	StatusSubDocNotJson      = StatusCode(0xc6)
	StatusSubDocBadRange     = StatusCode(0xc7)
	StatusSubDocBadDelta     = StatusCode(0xc8)
	StatusSubDocPathExists   = StatusCode(0xc9)
	StatusSubDocValueTooDeep = StatusCode(0xca)
	StatusSubDocBadCombo     = StatusCode(0xcb)
	StatusSubDocBadMulti     = StatusCode(0xcc)
)

type KeyState uint8

const (
	KeyStateNotPersisted = KeyState(0x00)
	KeyStatePersisted    = KeyState(0x01)
	KeyStateNotFound     = KeyState(0x80)
	KeyStateDeleted      = KeyState(0x81)
)

type StreamEndStatus uint32

const (
	StreamEndOK           = StreamEndStatus(0x00)
	StreamEndClosed       = StreamEndStatus(0x01)
	StreamEndStateChanged = StreamEndStatus(0x02)
	StreamEndDisconnected = StreamEndStatus(0x03)
	StreamEndTooSlow      = StreamEndStatus(0x04)
)

type BucketType int

const (
	BktTypeInvalid   BucketType = 0
	BktTypeCouchbase            = iota
	BktTypeMemcached            = iota
)

type VBucketState uint32

const (
	VBucketStateActive  = VBucketState(0x01)
	VBucketStateReplica = VBucketState(0x02)
	VBucketStatePending = VBucketState(0x03)
	VBucketStateDead    = VBucketState(0x04)
)

type SetMetaOption uint32

const (
	SkipConflictResolution   = SetMetaOption(0x01)
	UseLwwConflictResolution = SetMetaOption(0x02)
	RegenerateCas            = SetMetaOption(0x04)
)
