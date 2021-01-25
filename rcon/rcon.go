package rcon

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

const (
	cmdAuth        = 3
	cmdExecCommand = 2

	respResponse     = 0
	respAuthResponse = 2
)

// 12 byte header, up to 4096 bytes of data, 2 bytes for null terminators.
// this should be the absolute max size of a single response.
const (
	readBufferSize = 4110
	reqBaseID      = 0x7fffffff
)

type RemoteConsole struct {
	conn      net.Conn
	readBuf   []byte
	readMu    sync.Mutex
	reqID     int32
	queuedBuf []byte
}

var (
	ErrAuthFailed          = errors.New("rcon: authentication failed")
	ErrInvalidAuthResponse = errors.New("rcon: invalid response type during auth")
	ErrUnexpectedFormat    = errors.New("rcon: unexpected response format")
	ErrCommandTooLong      = errors.New("rcon: command too long")
	ErrResponseTooLong     = errors.New("rcon: response too long")
)

var BuildVersion = "master"

func Dial(ctx context.Context, host, password string, timeout time.Duration) (*RemoteConsole, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to connect to remote server")
	}
	var reqID int
	r := &RemoteConsole{conn: conn, reqID: reqBaseID}
	reqID, err = r.writeCmd(cmdAuth, password)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to write to remote host")
	}

	r.readBuf = make([]byte, readBufferSize)

	var respType, requestID int
	respType, requestID, _, err = r.readResponse(timeout)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read response from remote host")
	}

	// if we didn't get an auth response back, try again. it is often a bug
	// with RCON servers that you get an empty response before receiving the
	// auth response.
	if respType != respAuthResponse {
		respType, requestID, _, err = r.readResponse(timeout)
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read response")
	}
	if respType != respAuthResponse {
		return nil, errors.Wrap(ErrInvalidAuthResponse, "Invalid authentication response")
	}
	if requestID != reqID {
		return nil, errors.Wrap(ErrAuthFailed, "Invalid authentication")
	}

	return r, nil
}

func (r *RemoteConsole) LocalAddr() net.Addr {
	return r.conn.LocalAddr()
}

func (r *RemoteConsole) RemoteAddr() net.Addr {
	return r.conn.RemoteAddr()
}

func (r *RemoteConsole) Write(cmd string) (requestID int, err error) {
	return r.writeCmd(cmdExecCommand, cmd)
}

func (r *RemoteConsole) Read() (response string, requestID int, err error) {
	var respType int
	var respBytes []byte
	const respTimeout = 2
	respType, requestID, respBytes, err = r.readResponse(respTimeout * time.Minute)
	if err != nil || respType != respResponse {
		response = ""
		requestID = 0
	} else {
		response = string(respBytes)
	}

	return
}

func (r *RemoteConsole) Close() error {
	return r.conn.Close()
}

func newRequestID(id int32) int32 {
	const (
		checkID = 0x0fffffff
		timeDiv = 100000
	)
	if id&checkID != id {
		return int32((time.Now().UnixNano() / timeDiv) % timeDiv)
	}

	return id + 1
}

func (r *RemoteConsole) writeCmd(cmdType int32, str string) (int, error) {
	const (
		cmdPrefixSize = 14
		packetPrefix  = 10
		timeout       = 10 * time.Second
	)
	if len(str) > 1024-10 {
		return -1, errors.Wrap(ErrCommandTooLong, "Command too long")
	}
	buffer := bytes.NewBuffer(make([]byte, 0, cmdPrefixSize+len(str)))
	reqID := atomic.LoadInt32(&r.reqID)
	reqID = newRequestID(reqID)
	atomic.StoreInt32(&r.reqID, reqID)

	// packet size
	if err := binary.Write(buffer, binary.LittleEndian, int32(packetPrefix+len(str))); err != nil {
		return 0, errors.Wrap(err, "Failed to write packet")
	}

	// request id
	if err := binary.Write(buffer, binary.LittleEndian, reqID); err != nil {
		return 0, errors.Wrap(err, "Failed to write packet")
	}

	// auth cmd
	if err := binary.Write(buffer, binary.LittleEndian, cmdType); err != nil {
		return 0, errors.Wrap(err, "Failed to write packet")
	}

	// string (null terminated)
	buffer.WriteString(str)
	if err := binary.Write(buffer, binary.LittleEndian, byte(0)); err != nil {
		return 0, errors.Wrap(err, "Failed to write packet")
	}

	// string 2 (null terminated)
	// we don't have a use for string 2
	if err := binary.Write(buffer, binary.LittleEndian, byte(0)); err != nil {
		return 0, errors.Wrap(err, "Failed to write packet")
	}

	if err := r.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return 0, errors.Wrap(err, "Failed to set write deadline")
	}
	_, err := r.conn.Write(buffer.Bytes())
	if err != nil {
		return 0, errors.Wrap(err, "Failed to write packet")
	}

	return int(reqID), nil
}

func (r *RemoteConsole) readResponse(timeout time.Duration) (respType int, reqID int, body []byte, err error) {
	const (
		maxRespSize = 4106
		packetSize  = 4
		formatSize  = 10
	)
	r.readMu.Lock()
	defer r.readMu.Unlock()

	if err = r.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, 0, nil, errors.Wrap(err, "Failed to set read deadline")
	}
	var size int
	if r.queuedBuf != nil {
		copy(r.readBuf, r.queuedBuf)
		size = len(r.queuedBuf)
		r.queuedBuf = nil
	} else {
		size, err = r.conn.Read(r.readBuf)
		if err != nil {
			return 0, 0, nil, errors.Wrap(err, "Failed to read response")
		}
	}
	if size < packetSize {
		// need the 4 byte packet size...
		s, err2 := r.conn.Read(r.readBuf[size:])
		if err2 != nil {
			return 0, 0, nil, errors.Wrap(err2, "Failed to read response")
		}
		size += s
	}

	var dataSize32 int32
	b := bytes.NewBuffer(r.readBuf[:size])
	if err = binary.Read(b, binary.LittleEndian, &dataSize32); err != nil {
		return 0, 0, nil, errors.Wrap(err, "Failed to read response data")
	}
	if dataSize32 < formatSize {
		return 0, 0, nil, errors.Wrap(ErrUnexpectedFormat, "Unexpected format read")
	}

	totalSize := size
	dataSize := int(dataSize32)
	if dataSize > maxRespSize {
		return 0, 0, nil, errors.Wrap(ErrResponseTooLong, "response too long")
	}

	for dataSize+4 > totalSize {
		size, err = r.conn.Read(r.readBuf[totalSize:])
		if err != nil {
			return 0, 0, nil, errors.Wrap(err, "error reading from connection")
		}
		totalSize += size
	}

	data := r.readBuf[4 : 4+dataSize]
	if totalSize > dataSize+4 {
		// start of the next buffer was at the end of this packet.
		// save it for the next read.
		r.queuedBuf = r.readBuf[4+dataSize : totalSize]
	}

	return r.readResponseData(data)
}

func (r *RemoteConsole) readResponseData(data []byte) (respType int, reqID int, body []byte, err error) {
	const (
		delimiter = 0x00
	)
	var requestID, responseType int32
	var response []byte
	b := bytes.NewBuffer(data)
	if err = binary.Read(b, binary.LittleEndian, &requestID); err != nil {
		return 0, 0, nil, errors.Wrap(err, "Failed to read request id")
	}
	if err = binary.Read(b, binary.LittleEndian, &responseType); err != nil {
		return 0, 0, nil, errors.Wrap(err, "Failed to read response type")
	}
	response, err = b.ReadBytes(delimiter)
	if err != nil && errors.Is(err, io.EOF) {
		return 0, 0, nil, errors.Wrap(err, "Failed to read response body")
	}
	if err == nil {
		// if we didn't hit EOF, we have a null byte to remove
		response = response[:len(response)-1]
	}

	return int(responseType), int(requestID), response, nil
}

func (r *RemoteConsole) Exec(c string) (string, error) {
	wid, err := r.Write(c)
	if err != nil {
		log.Fatalf("Failed to write RCON command")
	}
	for {
		resp, rid, err := r.Read()
		if err != nil {
			log.Fatalf("Failed to read response")
		}
		if wid == rid {
			return resp, nil
		}
	}
}
