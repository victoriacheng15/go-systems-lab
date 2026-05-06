package main

import (
	"encoding/binary"
	"strings"
	"syscall"
	"testing"
)

func TestParseRouteAttrs(t *testing.T) {
	data := appendRouteAttr(nil, syscall.IFLA_IFNAME, []byte("eth0\x00"))
	data = appendRouteAttr(data, syscall.RTA_DST, []byte{203, 0, 113, 0})

	attrs, err := parseRouteAttrs(data)
	if err != nil {
		t.Fatalf("parseRouteAttrs returned error: %v", err)
	}
	if len(attrs) != 2 {
		t.Fatalf("len(attrs) = %d, want 2", len(attrs))
	}
	if got := attrString(attrs, syscall.IFLA_IFNAME); got != "eth0" {
		t.Fatalf("interface attr = %q, want eth0", got)
	}
	if got := attrIP(attrs, syscall.RTA_DST, syscall.AF_INET); got != "203.0.113.0" {
		t.Fatalf("route destination = %q, want 203.0.113.0", got)
	}
}

func TestFormatLinkEvent(t *testing.T) {
	data := encodeIfInfoMsg(syscall.AF_UNSPEC, 772, 1, syscall.IFF_UP, 0)
	data = appendRouteAttr(data, syscall.IFLA_IFNAME, []byte("lo\x00"))

	event, err := formatNetlinkEvent(syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: syscall.RTM_NEWLINK},
		Data:   data,
	})
	if err != nil {
		t.Fatalf("formatNetlinkEvent returned error: %v", err)
	}

	for _, want := range []string{"event=RTM_NEWLINK", "interface=lo", "index=1", "state=up"} {
		if !strings.Contains(event, want) {
			t.Fatalf("event %q does not contain %q", event, want)
		}
	}
}

func TestFormatAddrEvent(t *testing.T) {
	data := encodeIfAddrMsg(syscall.AF_INET, 32, 0, 0, 1)
	data = appendRouteAttr(data, syscall.IFA_LOCAL, []byte{127, 0, 0, 42})

	event, err := formatNetlinkEvent(syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: syscall.RTM_NEWADDR},
		Data:   data,
	})
	if err != nil {
		t.Fatalf("formatNetlinkEvent returned error: %v", err)
	}

	for _, want := range []string{"event=RTM_NEWADDR", "ifindex=1", "address=127.0.0.42/32", "family=ipv4"} {
		if !strings.Contains(event, want) {
			t.Fatalf("event %q does not contain %q", event, want)
		}
	}
}

func TestFormatRouteEvent(t *testing.T) {
	data := encodeRTMsg(syscall.AF_INET, 24, 254, 0, 0, 1, 0)
	data = appendRouteAttr(data, syscall.RTA_DST, []byte{203, 0, 113, 0})

	event, err := formatNetlinkEvent(syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: syscall.RTM_NEWROUTE},
		Data:   data,
	})
	if err != nil {
		t.Fatalf("formatNetlinkEvent returned error: %v", err)
	}

	for _, want := range []string{"event=RTM_NEWROUTE", "dst=203.0.113.0/24", "family=ipv4", "table=254"} {
		if !strings.Contains(event, want) {
			t.Fatalf("event %q does not contain %q", event, want)
		}
	}
}

func TestParseRouteAttrsRejectsBadLength(t *testing.T) {
	data := make([]byte, rtAttrLen)
	binary.NativeEndian.PutUint16(data[0:2], 2)
	binary.NativeEndian.PutUint16(data[2:4], syscall.IFLA_IFNAME)

	_, err := parseRouteAttrs(data)
	if err == nil {
		t.Fatal("parseRouteAttrs succeeded for invalid length")
	}
}

func TestParseRouteAttrsRejectsMissingPadding(t *testing.T) {
	data := []byte{5, 0, byte(syscall.IFLA_IFNAME), 0, 'x'}

	_, err := parseRouteAttrs(data)
	if err == nil {
		t.Fatal("parseRouteAttrs succeeded for missing padding")
	}
}

func appendRouteAttr(dst []byte, attrType uint16, payload []byte) []byte {
	length := rtAttrLen + len(payload)
	aligned := nlAlign(length)
	start := len(dst)
	for i := 0; i < aligned; i++ {
		dst = append(dst, 0)
	}
	binary.NativeEndian.PutUint16(dst[start:start+2], uint16(length))
	binary.NativeEndian.PutUint16(dst[start+2:start+4], attrType)
	copy(dst[start+rtAttrLen:start+length], payload)
	return dst
}

func encodeIfInfoMsg(family uint8, typ uint16, index int32, flags, change uint32) []byte {
	data := make([]byte, ifInfoMsgLen)
	data[0] = family
	binary.NativeEndian.PutUint16(data[2:4], typ)
	binary.NativeEndian.PutUint32(data[4:8], uint32(index))
	binary.NativeEndian.PutUint32(data[8:12], flags)
	binary.NativeEndian.PutUint32(data[12:16], change)
	return data
}

func encodeIfAddrMsg(family, prefixLen, flags, scope uint8, index uint32) []byte {
	data := make([]byte, ifAddrMsgLen)
	data[0] = family
	data[1] = prefixLen
	data[2] = flags
	data[3] = scope
	binary.NativeEndian.PutUint32(data[4:8], index)
	return data
}

func encodeRTMsg(family, dstLen, table, protocol, scope, routeType uint8, flags uint32) []byte {
	data := make([]byte, rtMsgLen)
	data[0] = family
	data[1] = dstLen
	data[4] = table
	data[5] = protocol
	data[6] = scope
	data[7] = routeType
	binary.NativeEndian.PutUint32(data[8:12], flags)
	return data
}
