// Copyright © 2021 The Gomon Project.

package network

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// interfaces captures system's network interfaces.
func interfaces() (ms []*measurement) {
	nis, err := net.Interfaces()
	if err != nil {
		core.LogError(err)
		return
	}

	for _, ni := range nis {
		addrs, err := ni.Addrs()
		if err != nil {
			continue
		}

		var address, broadcast, linklocal6, netmask string
		var address6 []string
		for _, addr := range addrs {
			ip, ipnet, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}

			if ip.To4() != nil {
				address = ip.String()
				netmask = ipnet.Mask.String()
				if ni.Flags&net.FlagLoopback != 0 {
					broadcast = "0.0.0.0"
				} else {
					bcast := binary.BigEndian.Uint32(ip.To4()) | ^binary.BigEndian.Uint32(ipnet.Mask)
					buf := make([]byte, 4)
					binary.BigEndian.PutUint32(buf, bcast)
					broadcast = net.IP(buf).String()
				}
			} else if ip.To16() != nil {
				prefix, _ := ipnet.Mask.Size()
				addr := ip.String() + "/" + strconv.Itoa(prefix)
				if ip.IsLinkLocalUnicast() {
					linklocal6 = addr
				} else {
					address6 = append(address6, addr)
				}
			}
		}

		ms = append(ms, &measurement{
			Header: message.Measurement(sourceNetwork),
			Id: Id{
				Name: ni.Name,
			},
			Properties: Properties{
				Index:      ni.Index,
				Flags:      ni.Flags.String(),
				Mac:        ni.HardwareAddr.String(),
				Mtu:        ni.MTU,
				Address:    address,
				Netmask:    netmask,
				Broadcast:  broadcast,
				Linklocal6: linklocal6,
				Address6:   strings.Join(address6, " "),
			},
		})
	}

	return
}
