package gocbcore

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"time"
)

// The data for a request that can be queued with a memdqueueconn,
//   and can potentially be rerouted to multiple servers due to
//   configuration changes.
type memdRequest struct {
	Magic    CommandMagic
	Opcode   CommandCode
	Datatype uint8
	Vbucket  uint16
	Opaque   uint32
	Cas      uint64
	Key      []byte
	Extras   []byte
	Value    []byte
}

// The data returned from the server in relation to an executed
//   request.
type memdResponse struct {
	Magic    CommandMagic
	Opcode   CommandCode
	Datatype uint8
	Status   StatusCode
	Opaque   uint32
	Cas      uint64
	Key      []byte
	Extras   []byte
	Value    []byte
}

type memdDialer interface {
	Dial(address string) (io.ReadWriteCloser, error)
}

type memdReadWriteCloser interface {
	WritePacket(*memdRequest) error
	ReadPacket(*memdResponse) error
	Close() error
}

type memdConn struct {
	conn    io.ReadWriteCloser
	recvBuf []byte
}

func DialMemdConn(address string, tlsConfig *tls.Config, deadline time.Time) (*memdConn, error) {
	d := net.Dialer{
		Deadline: deadline,
	}

	baseConn, err := d.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	tcpConn := baseConn.(*net.TCPConn)
	tcpConn.SetNoDelay(false)

	var conn io.ReadWriteCloser
	if tlsConfig == nil {
		conn = tcpConn
	} else {
		tlsConn := tls.Client(tcpConn, tlsConfig)
		err = tlsConn.Handshake()
		if err != nil {
			return nil, err
		}

		conn = tlsConn
	}

	return &memdConn{
		conn: conn,
	}, nil
}

func (s *memdConn) Close() error {
	return s.conn.Close()
}

func (s *memdConn) WritePacket(req *memdRequest) error {
	extLen := len(req.Extras)
	keyLen := len(req.Key)
	valLen := len(req.Value)

	// Go appears to do some clever things in regards to writing data
	//   to the kernel for network dispatch.  Having a write buffer
	//   per-server that is re-used actually hinders performance...
	// For now, we will simply create a new buffer and let it be GC'd.
	buffer := make([]byte, 24+keyLen+extLen+valLen)

	buffer[0] = uint8(req.Magic)
	buffer[1] = uint8(req.Opcode)
	binary.BigEndian.PutUint16(buffer[2:], uint16(keyLen))
	buffer[4] = byte(extLen)
	buffer[5] = req.Datatype
	binary.BigEndian.PutUint16(buffer[6:], uint16(req.Vbucket))
	binary.BigEndian.PutUint32(buffer[8:], uint32(len(buffer)-24))
	binary.BigEndian.PutUint32(buffer[12:], req.Opaque)
	binary.BigEndian.PutUint64(buffer[16:], req.Cas)

	copy(buffer[24:], req.Extras)
	copy(buffer[24+extLen:], req.Key)
	copy(buffer[24+extLen+keyLen:], req.Value)

	_, err := s.conn.Write(buffer)
	return err
}

func (s *memdConn) readBuffered(n int) ([]byte, error) {
	// Make sure our buffer is big enough to hold all our data
	if len(s.recvBuf) < n {
		neededSize := 4096
		if neededSize < n {
			neededSize = n
		}
		newBuf := make([]byte, neededSize)
		copy(newBuf[0:], s.recvBuf[0:])
		s.recvBuf = newBuf[0:len(s.recvBuf)]
	}

	// Loop till we encounter an error or have enough data...
	for {
		// Check if we already have enough data buffered
		if n <= len(s.recvBuf) {
			buf := s.recvBuf[0:n]
			s.recvBuf = s.recvBuf[n:]
			return buf, nil
		}

		// Read data up to the capacity
		recvTgt := s.recvBuf[len(s.recvBuf):cap(s.recvBuf)]
		n, err := s.conn.Read(recvTgt)
		if n <= 0 {
			return nil, err
		}

		// Update the len of our slice to encompass our new data
		s.recvBuf = s.recvBuf[:len(s.recvBuf)+n]
	}
}

func (s *memdConn) ReadPacket(resp *memdResponse) error {
	hdrBuf, err := s.readBuffered(24)
	if err != nil {
		return err
	}

	bodyLen := int(binary.BigEndian.Uint32(hdrBuf[8:]))
	bodyBuf, err := s.readBuffered(bodyLen)
	if err != nil {
		return err
	}

	keyLen := int(binary.BigEndian.Uint16(hdrBuf[2:]))
	extLen := int(hdrBuf[4])

	resp.Magic = CommandMagic(hdrBuf[0])
	resp.Opcode = CommandCode(hdrBuf[1])
	resp.Datatype = hdrBuf[5]
	resp.Status = StatusCode(binary.BigEndian.Uint16(hdrBuf[6:]))
	resp.Opaque = binary.BigEndian.Uint32(hdrBuf[12:])
	resp.Cas = binary.BigEndian.Uint64(hdrBuf[16:])
	resp.Extras = bodyBuf[:extLen]
	resp.Key = bodyBuf[extLen : extLen+keyLen]
	resp.Value = bodyBuf[extLen+keyLen:]
	return nil
}
