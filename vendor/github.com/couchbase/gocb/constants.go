package gocb

const (
	// Legacy flag format for JSON data.
	lfJson = 0

	// Common flags mask
	cfMask = 0xFF000000
	// Common flags mask for data format
	cfFmtMask = 0x0F000000
	// Common flags mask for compression mode.
	cfCmprMask = 0xE0000000

	// Common flag format for sdk-private data.
	cfFmtPrivate = 1 << 24
	// Common flag format for JSON data.
	cfFmtJson = 2 << 24
	// Common flag format for binary data.
	cfFmtBinary = 3 << 24
	// Common flag format for string data.
	cfFmtString = 4 << 24

	// Common flags compression for disabled compression.
	cfCmprNone = 0 << 29
)

// IndexType provides information on the type of indexer used for an index.
type IndexType string

const (
	// IndexTypeN1ql indicates that GSI was used to build the index.
	IndexTypeN1ql = IndexType("gsi")

	// IndexTypeView indicates that views were used to build the index.
	IndexTypeView = IndexType("views")
)
