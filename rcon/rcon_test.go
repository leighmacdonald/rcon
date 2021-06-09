package rcon

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func startTestServer(fn func(net.Conn, *bytes.Buffer)) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() {
			_ = conn.Close()
		}()

		buf := make([]byte, readBufferSize)
		_, err = conn.Read(buf)
		if err != nil {
			return
		}

		var packetSize, requestId, cmdType int32
		var str []byte
		b := bytes.NewBuffer(buf)
		var re error
		re = binary.Read(b, binary.LittleEndian, &packetSize)
		if re != nil {
			return
		}
		re = binary.Read(b, binary.LittleEndian, &requestId)
		if re != nil {
			return
		}
		re = binary.Read(b, binary.LittleEndian, &cmdType)
		if re != nil {
			return
		}
		str, err = b.ReadBytes(0x00)
		if err != nil {
			return
		}
		if string(str[:len(str)-1]) != "blerg" {
			requestId = -1
		}

		b.Reset()
		re = binary.Write(b, binary.LittleEndian, int32(10))
		if re != nil {
			return
		}
		re = binary.Write(b, binary.LittleEndian, requestId)
		if re != nil {
			return
		}
		re = binary.Write(b, binary.LittleEndian, int32(respAuthResponse))
		if re != nil {
			return
		}
		re = binary.Write(b, binary.LittleEndian, byte(0))
		if re != nil {
			return
		}
		re = binary.Write(b, binary.LittleEndian, byte(0))
		if re != nil {
			return
		}
		_, re = conn.Write(b.Bytes())
		if re != nil {
			return
		}

		if fn != nil {
			b.Reset()
			fn(conn, b)
		}
	}()

	return listener.Addr().String(), nil
}

func TestAuth(t *testing.T) {
	addr, err := startTestServer(nil)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := Dial(context.Background(), addr, "blerg", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = rc.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultipacket(t *testing.T) {
	addr, err := startTestServer(func(c net.Conn, b *bytes.Buffer) {
		// start packet
		// start response
		var we error
		we = binary.Write(b, binary.LittleEndian, int32(10+4000))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(123))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(respResponse))
		if we != nil {
			return
		}
		for i := 0; i < 4000; i += 1 {
			we = binary.Write(b, binary.LittleEndian, byte(' '))
			if we != nil {
				return
			}
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		// end response
		// start response
		we = binary.Write(b, binary.LittleEndian, int32(10+4000))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(123))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(respResponse))
		if we != nil {
			return
		}
		for i := 0; i < 2000; i += 1 {
			we = binary.Write(b, binary.LittleEndian, byte(' '))
			if we != nil {
				return
			}
		}
		_, we = c.Write(b.Bytes())
		if we != nil {
			return
		}
		// end packet

		// start packet
		b.Reset()
		for i := 0; i < 2000; i += 1 {
			we = binary.Write(b, binary.LittleEndian, byte(' '))
			if we != nil {
				return
			}
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		// end response
		// start response
		we = binary.Write(b, binary.LittleEndian, int32(10+2000))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(123))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(respResponse))
		if we != nil {
			return
		}

		for i := 0; i < 2000; i += 1 {
			we = binary.Write(b, binary.LittleEndian, byte(' '))
			if we != nil {
				return
			}
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		// end response
		// start response - size word is split!
		we = binary.Write(b, binary.LittleEndian, int32(10+2000))
		if we != nil {
			return
		}
		_, we = c.Write(b.Bytes()[:len(b.Bytes())-3])
		if we != nil {
			return
		}
		// end packet

		b.Reset()
		we = binary.Write(b, binary.LittleEndian, int32(10+2000))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(123))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, int32(respResponse))
		if we != nil {
			return
		}
		for i := 0; i < 2000; i += 1 {
			we = binary.Write(b, binary.LittleEndian, byte(' '))
			if we != nil {
				return
			}
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		we = binary.Write(b, binary.LittleEndian, byte(0))
		if we != nil {
			return
		}
		// end response
		_, we = c.Write(b.Bytes()[1:])
		if we != nil {
			return
		}
		// end packet
	})
	if err != nil {
		t.Fatal(err)
	}

	rc, err := Dial(context.Background(), addr, "blerg", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	str, _, err := rc.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(str) != 4000 {
		t.Fatal("response length not correct")
	}

	str, _, err = rc.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(str) != 4000 {
		t.Fatal("response length not correct")
	}

	str, _, err = rc.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(str) != 2000 {
		t.Fatal("response length not correct")
	}

	str, _, err = rc.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(str) != 2000 {
		t.Fatal("response length not correct")
	}

	err = rc.Close()
	if err != nil {
		t.Fatal(err)
	}
}
