package msfixture

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestADMLookup(t *testing.T) {
	const recordStart = messageHeaderSize + admHeaderSize
	request := make([]byte, recordStart+admRecordSize*admRecordCount)
	copy(request, messageMagic)
	request[12], request[66], request[67] = 4, 4, 5
	copy(request[110:122], admMagic)
	request[122], request[123] = 1, 1
	copy(request[124:146], "        104          2")
	for i := range admRecordCount {
		record := request[recordStart+i*admRecordSize:]
		record[0], record[4], record[5], record[7] = admReadText, 0x16, 20, 40
		copy(record[8:28], "RFC_GROUP           ")
	}

	reply, err := lookup(request)
	if err != nil {
		t.Fatal(err)
	}

	record := reply[recordStart:]
	if reply[123] != 2 || record[2] != 1 || record[3] != 0 ||
		string(record[28:37]) != admGroupTarget || record[37] != 1 ||
		!bytes.Equal(record[38:42], []byte{127, 0, 0, 1}) ||
		binary.BigEndian.Uint16(record[46:48]) != DispatcherPort {
		t.Fatalf("invalid ADM reply %x", reply[messageHeaderSize:])
	}

	copy(request[recordStart+8:], "BAD_GROUP")
	reply, err = lookup(request)
	if err != nil || len(reply) != len(request) || reply[recordStart+3] == 0 {
		t.Fatalf("unknown group was not rejected: %x, %v", reply, err)
	}

	if _, err := lookup(request[:len(request)-1]); err == nil {
		t.Fatal("truncated ADM lookup was accepted")
	}

	request[recordStart] = 0x16
	if _, err := lookup(request); err == nil {
		t.Fatal("unsupported ADM record was accepted")
	}
}

func TestGroupLookup(t *testing.T) {
	request := make([]byte, groupRequestSize)
	request[66], request[67] = 2, 1
	request[110], request[112] = lookupGroup, 1
	copy(request[158:198], Group)

	reply, err := lookup(request)
	if err != nil {
		t.Fatal(err)
	}

	if reply[111] != 0 || binary.BigEndian.Uint16(reply[202:204]) != DispatcherPort ||
		binary.BigEndian.Uint16(reply[204:206]) != uint16(len(Host)) || string(reply[206:]) != Host+" " {
		t.Fatalf("invalid group reply %x", reply[messageHeaderSize:])
	}

	clear(request[158:198])
	reply, err = lookup(request)
	if err != nil || reply[111] == 0 {
		t.Fatalf("unknown group was not rejected: %x, %v", reply, err)
	}
}
