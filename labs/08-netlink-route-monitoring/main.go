package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const (
	rtmGrpLink       = 0x1
	rtmGrpIPv4IfAddr = 0x10
	rtmGrpIPv4Route  = 0x40
	rtmGrpIPv6IfAddr = 0x100
	rtmGrpIPv6Route  = 0x400

	ifInfoMsgLen = 16
	ifAddrMsgLen = 8
	rtMsgLen     = 12
	rtAttrLen    = 4
)

type ifInfoMsg struct {
	Family uint8
	Type   uint16
	Index  int32
	Flags  uint32
	Change uint32
}

type ifAddrMsg struct {
	Family    uint8
	PrefixLen uint8
	Flags     uint8
	Scope     uint8
	Index     uint32
}

type rtMsg struct {
	Family   uint8
	DstLen   uint8
	SrcLen   uint8
	TOS      uint8
	Table    uint8
	Protocol uint8
	Scope    uint8
	Type     uint8
	Flags    uint32
}

type routeAttr struct {
	Type uint16
	Data []byte
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "monitor" {
		fmt.Println("Usage:")
		fmt.Println("  go run labs/08-netlink-route-monitoring/main.go monitor")
		os.Exit(1)
	}

	if err := monitor(); err != nil {
		fmt.Fprintf(os.Stderr, "monitor: %v\n", err)
		os.Exit(1)
	}
}

func monitor() error {
	fd, err := openRouteSocket()
	if err != nil {
		return err
	}
	defer syscall.Close(fd)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	errs := make(chan error, 1)
	go func() {
		errs <- readEvents(fd)
	}()

	fmt.Println("listening for netlink route events; press Ctrl+C to stop")

	select {
	case <-stop:
		return nil
	case err := <-errs:
		return err
	}
}

func openRouteSocket() (int, error) {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_ROUTE)
	if err != nil {
		return -1, err
	}

	groups := rtmGrpLink | rtmGrpIPv4IfAddr | rtmGrpIPv6IfAddr | rtmGrpIPv4Route | rtmGrpIPv6Route
	addr := &syscall.SockaddrNetlink{
		Family: syscall.AF_NETLINK,
		Groups: uint32(groups),
	}

	if err := syscall.Bind(fd, addr); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	return fd, nil
}

func readEvents(fd int) error {
	buf := make([]byte, 1<<16)

	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return err
		}

		msgs, err := syscall.ParseNetlinkMessage(buf[:n])
		if err != nil {
			return err
		}

		for _, msg := range msgs {
			event, err := formatNetlinkEvent(msg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "decode message type %d: %v\n", msg.Header.Type, err)
				continue
			}
			if event != "" {
				fmt.Println(event)
			}
		}
	}
}

func formatNetlinkEvent(msg syscall.NetlinkMessage) (string, error) {
	switch msg.Header.Type {
	case syscall.RTM_NEWLINK, syscall.RTM_DELLINK:
		return formatLinkEvent(msg)
	case syscall.RTM_NEWADDR, syscall.RTM_DELADDR:
		return formatAddrEvent(msg)
	case syscall.RTM_NEWROUTE, syscall.RTM_DELROUTE:
		return formatRouteEvent(msg)
	case syscall.NLMSG_DONE:
		return "", nil
	case syscall.NLMSG_ERROR:
		return "netlink error message received", nil
	default:
		return fmt.Sprintf("event=%s bytes=%d", routeMessageName(msg.Header.Type), len(msg.Data)), nil
	}
}

func formatLinkEvent(msg syscall.NetlinkMessage) (string, error) {
	info, err := parseIfInfoMsg(msg.Data)
	if err != nil {
		return "", err
	}
	attrs, err := parseRouteAttrs(msg.Data[ifInfoMsgLen:])
	if err != nil {
		return "", err
	}

	name := attrString(attrs, syscall.IFLA_IFNAME)
	if name == "" {
		name = fmt.Sprintf("ifindex:%d", info.Index)
	}

	state := "down"
	if info.Flags&syscall.IFF_UP != 0 {
		state = "up"
	}

	return fmt.Sprintf("event=%s interface=%s index=%d state=%s flags=0x%x", routeMessageName(msg.Header.Type), name, info.Index, state, info.Flags), nil
}

func formatAddrEvent(msg syscall.NetlinkMessage) (string, error) {
	addr, err := parseIfAddrMsg(msg.Data)
	if err != nil {
		return "", err
	}
	attrs, err := parseRouteAttrs(msg.Data[ifAddrMsgLen:])
	if err != nil {
		return "", err
	}

	ip := attrIP(attrs, syscall.IFA_LOCAL, addr.Family)
	if ip == "" {
		ip = attrIP(attrs, syscall.IFA_ADDRESS, addr.Family)
	}
	if ip == "" {
		ip = "<unknown>"
	}

	return fmt.Sprintf("event=%s ifindex=%d address=%s/%d family=%s", routeMessageName(msg.Header.Type), addr.Index, ip, addr.PrefixLen, addressFamilyName(addr.Family)), nil
}

func formatRouteEvent(msg syscall.NetlinkMessage) (string, error) {
	route, err := parseRTMsg(msg.Data)
	if err != nil {
		return "", err
	}
	attrs, err := parseRouteAttrs(msg.Data[rtMsgLen:])
	if err != nil {
		return "", err
	}

	dst := attrIP(attrs, syscall.RTA_DST, route.Family)
	if dst == "" {
		dst = "default"
	}

	return fmt.Sprintf("event=%s dst=%s/%d family=%s table=%d protocol=%d scope=%d", routeMessageName(msg.Header.Type), dst, route.DstLen, addressFamilyName(route.Family), route.Table, route.Protocol, route.Scope), nil
}

func parseIfInfoMsg(data []byte) (ifInfoMsg, error) {
	if len(data) < ifInfoMsgLen {
		return ifInfoMsg{}, fmt.Errorf("ifinfomsg too short: %d", len(data))
	}

	return ifInfoMsg{
		Family: data[0],
		Type:   binary.NativeEndian.Uint16(data[2:4]),
		Index:  int32(binary.NativeEndian.Uint32(data[4:8])),
		Flags:  binary.NativeEndian.Uint32(data[8:12]),
		Change: binary.NativeEndian.Uint32(data[12:16]),
	}, nil
}

func parseIfAddrMsg(data []byte) (ifAddrMsg, error) {
	if len(data) < ifAddrMsgLen {
		return ifAddrMsg{}, fmt.Errorf("ifaddrmsg too short: %d", len(data))
	}

	return ifAddrMsg{
		Family:    data[0],
		PrefixLen: data[1],
		Flags:     data[2],
		Scope:     data[3],
		Index:     binary.NativeEndian.Uint32(data[4:8]),
	}, nil
}

func parseRTMsg(data []byte) (rtMsg, error) {
	if len(data) < rtMsgLen {
		return rtMsg{}, fmt.Errorf("rtmsg too short: %d", len(data))
	}

	return rtMsg{
		Family:   data[0],
		DstLen:   data[1],
		SrcLen:   data[2],
		TOS:      data[3],
		Table:    data[4],
		Protocol: data[5],
		Scope:    data[6],
		Type:     data[7],
		Flags:    binary.NativeEndian.Uint32(data[8:12]),
	}, nil
}

func parseRouteAttrs(data []byte) ([]routeAttr, error) {
	var attrs []routeAttr

	for len(data) >= rtAttrLen {
		length := int(binary.NativeEndian.Uint16(data[0:2]))
		attrType := binary.NativeEndian.Uint16(data[2:4])
		if length < rtAttrLen {
			return nil, fmt.Errorf("invalid attribute length: %d", length)
		}
		if length > len(data) {
			return nil, fmt.Errorf("attribute length %d exceeds remaining bytes %d", length, len(data))
		}

		payload := make([]byte, length-rtAttrLen)
		copy(payload, data[rtAttrLen:length])
		attrs = append(attrs, routeAttr{Type: attrType, Data: payload})

		step := nlAlign(length)
		if step > len(data) {
			return nil, fmt.Errorf("attribute alignment %d exceeds remaining bytes %d", step, len(data))
		}
		data = data[step:]
	}

	return attrs, nil
}

func attrString(attrs []routeAttr, attrType uint16) string {
	for _, attr := range attrs {
		if attr.Type == attrType {
			return strings.TrimRight(string(attr.Data), "\x00")
		}
	}
	return ""
}

func attrIP(attrs []routeAttr, attrType uint16, family uint8) string {
	for _, attr := range attrs {
		if attr.Type != attrType {
			continue
		}
		switch family {
		case syscall.AF_INET:
			if ip := net.IP(attr.Data).To4(); ip != nil {
				return ip.String()
			}
		case syscall.AF_INET6:
			if len(attr.Data) >= net.IPv6len {
				return net.IP(attr.Data[:net.IPv6len]).String()
			}
		}
	}
	return ""
}

func routeMessageName(msgType uint16) string {
	switch msgType {
	case syscall.RTM_NEWLINK:
		return "RTM_NEWLINK"
	case syscall.RTM_DELLINK:
		return "RTM_DELLINK"
	case syscall.RTM_NEWADDR:
		return "RTM_NEWADDR"
	case syscall.RTM_DELADDR:
		return "RTM_DELADDR"
	case syscall.RTM_NEWROUTE:
		return "RTM_NEWROUTE"
	case syscall.RTM_DELROUTE:
		return "RTM_DELROUTE"
	default:
		return fmt.Sprintf("type:%d", msgType)
	}
}

func addressFamilyName(family uint8) string {
	switch family {
	case syscall.AF_INET:
		return "ipv4"
	case syscall.AF_INET6:
		return "ipv6"
	default:
		return fmt.Sprintf("family:%d", family)
	}
}

func nlAlign(length int) int {
	return (length + syscall.NLMSG_ALIGNTO - 1) & ^(syscall.NLMSG_ALIGNTO - 1)
}
