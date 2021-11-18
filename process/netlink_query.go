// Copyright © 2021 The Gomon Project.

//go:build linux
// +build linux

package process

import (
	"errors"
	"math"
	"net"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/zosmac/gomon/core"
)

/*
Linux Netlink API Reference:
	http://man7.org/linux/man-pages/man7/netlink.7.html
	http://man7.org/linux/man-pages/man7/sock_diag.7.html
	/usr/include/uapi/linux/netlink.h
	/usr/include/uapi/linux/rtnetlink.h
	/usr/include/uapi/linux/sock_diag.h
	/usr/include/uapi/linux/inet_diag.h
	/usr/include/uapi/linux/unix_diag.h

The INET and UNIX netlink queries rely on queries that are not available in older versions Linux.
It looks like these came in around Linux 3.3.
	inet_diag_req_v2 in /usr/include/uqpi/linux/inet_diag.h
	unix_diag_req in /usr/include/uqpi/linux/unix_diag.h
*/

// RTA_ALIGN from /usr/include/uapi/linux/rtnetlink.h
func rtaAlign(l int) int {
	return int((l + syscall.RTA_ALIGNTO - 1) &^ (syscall.RTA_ALIGNTO - 1))
}

// *******************************
// NETLINK convenience definitions
// http://man7.org/linux/man-pages/man7/netlink.7.html
// /usr/include/uapi/linux/netlink.h
// /usr/include/uapi/linux/rtnetlink.h
// /usr/include/uapi/linux/sock_diag.h
// *******************************

const (
	// SOCK_DIAG_BY_FAMILY is the netlink socket query key
	sockDiagByFamily = 20
)

// *************************************
// Netlink inet family query definitions
// *************************************

// INET queries use "extensions" to request additional information.
// We don't need this to get the connection endpoints
const (
	inetDiagReqNone = iota
	inetDiagReqBytecode
)

// INET_DIAG response types from /usr/include/linux/inet_diag.h
const (
	inetDiagNone = iota
	inetDiagMeminfo
	inetDiagIinfo
	inetDiagVegasInfo
	inetDiagCong
	inetDiagTOS
	inetDiagTClass
	inetDiagSKMeminfo
	inetDiagShutdown
	inetDiagDCTCPInfo
	inetDiagProtocol
	inetDiagSKV6Only
	inetDiagLocals
	inetDiagPeers
	inetDiagPad
	inetDiagMax = iota - 1
)

type (

	// inet_diag_req_v2 from /usr/include/linux/inet_diag.h
	inetDiagReqV2 struct {
		sdiagFamily   byte
		sdiagProtocol byte
		idiagExt      byte
		pad           byte
		idiagStates   uint32
		inetDiagSockID
	}

	// inet_diag_sockid from /usr/include/linux/inet_diag.h
	inetDiagSockID struct {
		idiagSport  uint16    // big endian
		idiagDport  uint16    // big endian
		idiagSrc    [4]uint32 // big endian
		idiagDst    [4]uint32 // big endian
		idiagIF     uint32
		idiagCookie [2]uint32
	}

	// inet_diag_msg from /usr/include/linux/inet_diag.h
	inetDiagMsg struct {
		idiagFamily  byte
		idiagState   byte
		idiagTimer   byte
		idiagRetrans byte
		inetDiagSockID
		idiagExpires uint32
		idiagRqueue  uint32
		idiagWqueue  uint32
		idiagUID     uint32
		idiagInode   uint32
	}

	// nlInetRequest defines netlink inet request header.
	nlInetRequest struct {
		syscall.NlMsghdr
		inetDiagReqV2
	}
)

// inetPeerInode queries netlink to find inet socket's remote connection inode based on its remote address.
// See http://man7.org/linux/man-pages/man7/sock_diag.7.html for API details.
func nlInetPeerInode(addr net.Addr) (int, error) {
	protocol := syscall.IPPROTO_TCP
	if addr.Network() == "udp" {
		protocol = syscall.IPPROTO_UDP
	}
	host, port, _ := net.SplitHostPort(addr.String())
	ip := net.ParseIP(host)
	family := syscall.AF_INET6
	if ip.To4() != nil {
		ip = ip.To4()
		family = syscall.AF_INET
	}

	p, _ := strconv.Atoi(port)
	sport := core.Htons(uint16(p))
	var src [4]uint32
	if len(ip) == net.IPv4len {
		src[0] = (*(*uint32)(unsafe.Pointer(&ip[0])))
	} else {
		for i := 0; i < 4; i++ {
			src[i] = ((*[4]uint32)(unsafe.Pointer(&ip[0])))[i]
		}
	}

	s, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_SOCK_DIAG)
	if err != nil {
		return 0, core.NewError("netlink socket", err)
	}
	defer syscall.Close(s)

	req := nlInetRequest{
		syscall.NlMsghdr{
			Len:   uint32(unsafe.Sizeof(nlInetRequest{})),
			Flags: syscall.NLM_F_REQUEST | syscall.NLM_F_MATCH,
			Type:  sockDiagByFamily,
			Seq:   1,
			Pid:   0,
		},
		inetDiagReqV2{
			sdiagFamily:   byte(family),
			sdiagProtocol: byte(protocol),
			inetDiagSockID: inetDiagSockID{
				idiagSrc:   src,
				idiagSport: sport,
			},
			idiagExt:    inetDiagReqNone,
			idiagStates: math.MaxUint32,
		},
	}

	buf := (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:]
	if err := syscall.Sendto(s, buf, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return 0, core.NewError("sendto", err)
	}

	for {
		nlMsg := make([]byte, 1024)
		n, _, err := syscall.Recvfrom(s, nlMsg, 0)
		if n <= 0 || err != nil {
			return 0, core.NewError("recvfrom", err)
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			switch m.Header.Type {
			case syscall.NLMSG_ERROR:
				err := syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4])))
				if errors.Is(err, syscall.EINVAL) {
					return 0, nil // ignore invalid argument, probably an old version of Linux
				}
				return 0, core.NewError("netlink", err)
			case syscall.NLMSG_DONE:
				return 0, nil
			case sockDiagByFamily:
				msg := (*inetDiagMsg)(unsafe.Pointer(&m.Data[0]))
				if src == msg.idiagSrc && sport == msg.idiagSport {
					return int(msg.idiagInode), nil
				}
			}
		}
	}
}

// *************************************
// Netlink unix family query definitions
// /usr/include/linux/unix_diag.h
// *************************************

// UDIAG_SHOW query types from /usr/include/linux/unix_diag.h
const (
	udiagShowName    = 0x00000001 /* show name (not path) */
	udiagShowVFS     = 0x00000002 /* show VFS inode info */
	udiagShowPeer    = 0x00000004 /* show peer socket info */
	udiagShowIcons   = 0x00000008 /* show pending connections */
	udiagShowRqlen   = 0x00000010 /* show skb receive queue len */
	udiagShowMeminfo = 0x00000020 /* show memory info of a socket */
)

// UNIX_DIAG response types
const (
	unixDiagName = iota
	unixDiagVFS
	unixDiagPeer
	unixDiagIcons
	unixDiagRqlen
	unixDiagMeminfo
	unixDiagShutdown
	unixDiagMax = iota - 1
)

// unix_diag_req from /usr/include/linux/unix_diag.h
type unixDiagReq struct {
	sdiagFamily   byte
	sdiagProtocol byte
	pad           uint16
	udiagStates   uint32
	udiagIno      uint32
	udiagShow     uint32
	udiagCookie   [2]uint32
}

// unix_diag_msg from /usr/include/linux/unix_diag.h
type unixDiagMsg struct {
	udiagFamily byte
	udiagType   byte
	udiagState  byte
	pad         byte
	udiagIno    uint32
	udiagCookie [2]uint32
}

// nlUnixRequest defines netlink unix request header.
type nlUnixRequest struct {
	syscall.NlMsghdr
	unixDiagReq
}

// unixPeerInode queries netlink to find a unix socket's remote connection inode connected to a local inode.
// See http://man7.org/linux/man-pages/man7/sock_diag.7.html for API details.
func nlUnixPeerInode(inode int) (int, error) {
	s, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_DGRAM, syscall.NETLINK_SOCK_DIAG)
	if err != nil {
		return 0, core.NewError("netlink socket", err)
	}
	defer syscall.Close(s)

	req := nlUnixRequest{
		syscall.NlMsghdr{
			Len:   uint32(unsafe.Sizeof(nlUnixRequest{})),
			Flags: syscall.NLM_F_REQUEST | syscall.NLM_F_MATCH,
			Type:  sockDiagByFamily,
			Seq:   1,
			Pid:   0,
		},
		unixDiagReq{
			sdiagFamily: syscall.AF_UNIX,
			udiagStates: math.MaxUint32,
			udiagIno:    uint32(inode),
			udiagShow:   udiagShowPeer,
		},
	}

	buf := (*[unsafe.Sizeof(req)]byte)(unsafe.Pointer(&req))[:]
	if err := syscall.Sendto(s, buf, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return 0, core.NewError("sendto", err)
	}

	for {
		nlMsg := make([]byte, 16384) // returns everything even with udiagIno set in query :(
		n, _, err := syscall.Recvfrom(s, nlMsg, 0)
		if n <= 0 || err != nil {
			return 0, core.NewError("recvfrom", err)
		}
		msgs, _ := syscall.ParseNetlinkMessage(nlMsg[:n])

		for _, m := range msgs {
			switch m.Header.Type {
			case syscall.NLMSG_ERROR:
				err := syscall.Errno(-int32(core.HostEndian.Uint32(m.Data[:4])))
				if errors.Is(err, syscall.EINVAL) {
					return 0, nil // ignore invalid argument, probably an old version of Linux
				}
				return 0, core.NewError("netlink", err)
			case syscall.NLMSG_DONE:
				return 0, nil
			case sockDiagByFamily:
				msg := (*unixDiagMsg)(unsafe.Pointer(&m.Data[0]))
				var attr *syscall.RtAttr
				for i := rtaAlign(int(unsafe.Sizeof(unixDiagMsg{}))); i < len(m.Data); i += rtaAlign(int(attr.Len)) {
					attr = (*syscall.RtAttr)(unsafe.Pointer(&m.Data[i]))
					if attr.Type == unixDiagPeer &&
						inode == int(core.HostEndian.Uint32(m.Data[i+4:i+int(attr.Len)])) {
						return int(msg.udiagIno), nil
					}
				}
			}
		}
	}
}
