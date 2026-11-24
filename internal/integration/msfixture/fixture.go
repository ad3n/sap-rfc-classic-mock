package msfixture

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"

	"time"
)

const (
	Group          = "RFC_GROUP"
	SystemID       = "DEV"
	Host           = "127.0.0.1"
	DispatcherPort = 3213
	GatewayPort    = DispatcherPort + 100

	messageMagic      = "**MESSAGE**\x00"
	messageHeaderSize = 110
	groupRequestSize  = 206
	admMagic          = "AD-EYECATCH\x00"
	admHeaderSize     = 36
	admRecordSize     = 104
	admRecordCount    = 2
	admReadText       = 0x15
	admGroupTarget    = "LG_EYECAT"
	lookupGroup       = 0x2c
	lookupHostname    = 0x44
	lookupAddress     = 0x46
)

func ServeConn(conn net.Conn) error {
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	login, err := receive(conn)
	if err != nil {
		return err
	}

	if len(login) != messageHeaderSize || !validHeader(login) || login[66] != 2 || login[67] != 8 {
		return errors.New("invalid message server login")
	}

	reply := replyHeader(login, true)
	if err := send(conn, reply); err != nil {
		return err
	}

	for {
		request, err := receive(conn)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return err
		}

		if !validHeader(request) {
			return errors.New("invalid message server header")
		}

		if len(request) == messageHeaderSize && request[67] == 4 {
			return nil
		}

		reply, err := lookup(request)
		if err != nil {
			return err
		}

		if err := send(conn, reply); err != nil {
			return err
		}
	}
}

func validHeader(message []byte) bool {
	return len(message) >= messageHeaderSize && string(message[:12]) == messageMagic && message[12] == 4
}

func replyHeader(request []byte, login bool) []byte {
	header := make([]byte, messageHeaderSize)
	copy(header, messageMagic)
	header[12] = 4
	copy(header[14:54], request[68:108])
	copy(header[68:108], "MSG_SERVER")
	header[66], header[67] = 3, request[67]
	if login {
		header[57], header[66] = 1, 2
		for i := 78; i < 108; i++ {
			header[i] = ' '
		}
	}

	return header
}

func lookup(request []byte) ([]byte, error) {
	switch {
	case request[66] == 4 && request[67] == 5:
		return admLookup(request)
	case request[66] == 2 && request[67] == 1 && len(request) == 137 && request[110] == lookupAddress:
		return unavailableHostname(request), nil
	case request[66] == 2 && request[67] == 1 && len(request) == groupRequestSize:
		return groupLookup(request)
	default:
		return nil, fmt.Errorf("unsupported message server request: length=%d flag=%d interface=%d", len(request), request[66], request[67])
	}
}

func admLookup(request []byte) ([]byte, error) {
	const recordStart = messageHeaderSize + admHeaderSize
	if len(request) != recordStart+admRecordSize*admRecordCount ||
		string(request[110:122]) != admMagic || request[122] != 1 || request[123] != 1 ||
		string(request[124:135]) != "        104" || string(request[135:146]) != "          2" {
		return nil, errors.New("unsupported ADM header")
	}

	records := request[recordStart:]
	for record := range slices.Chunk(records, admRecordSize) {
		if record[0] != admReadText || record[4] != 0x16 || record[5] != 20 || record[6] != 0 || record[7] != 40 {
			return nil, errors.New("unsupported ADM lookup record")
		}
	}

	reply := append(replyHeader(request, false), request[messageHeaderSize:]...)
	reply[123] = 2
	found := string(bytes.TrimRight(records[8:28], " ")) == Group
	for record := range slices.Chunk(reply[recordStart:], admRecordSize) {
		record[2] = 1
		clear(record[28:])
		if !found {
			record[3] = 1
			continue
		}

		copy(record[8:28], "RFC_GROUP           ")
		copy(record[28:37], admGroupTarget)
		record[37] = 1
		copy(record[38:42], []byte{127, 0, 0, 1})
		binary.BigEndian.PutUint16(record[46:48], DispatcherPort)
		copy(record[48:52], "13  ")
	}

	return reply, nil
}

func groupLookup(request []byte) ([]byte, error) {
	opcode := request[110]
	if (opcode != lookupGroup && opcode != lookupHostname) || request[112] != 1 {
		return nil, fmt.Errorf("unsupported lookup opcode %#x or version %d", opcode, request[112])
	}

	reply := append(replyHeader(request, false), request[messageHeaderSize:202]...)
	reply[113] = 3
	if string(bytes.TrimRight(request[158:198], "\x00")) != Group {
		reply[111] = 1
		return reply, nil
	}

	reply = binary.BigEndian.AppendUint16(reply, DispatcherPort)
	reply = binary.BigEndian.AppendUint16(reply, uint16(len(Host)))
	reply = append(reply, Host...)
	return append(reply, ' '), nil
}

func unavailableHostname(request []byte) []byte {
	reply := append(replyHeader(request, false), request[messageHeaderSize:]...)
	reply[111] = 0x13
	return reply
}

func receive(conn net.Conn) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(header[:])
	if size < messageHeaderSize || size > 512 {
		return nil, fmt.Errorf("unexpected message server frame length %d", size)
	}

	message := make([]byte, size)
	_, err := io.ReadFull(conn, message)
	return message, err
}

func send(conn net.Conn, message []byte) error {
	frame := binary.BigEndian.AppendUint32(make([]byte, 0, len(message)+4), uint32(len(message)))
	_, err := conn.Write(append(frame, message...))
	return err
}
