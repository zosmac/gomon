// Copyright © 2021 The Gomon Project.

package network

/*
#include <net/if.h>
#include <net/if_var.h>
#include <sys/sysctl.h>
*/
import "C"

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/message"
)

// Measure captures system's network interface metrics.
func Measure() (ms []message.Content) {
	for _, m := range interfaces() {
		var size C.size_t
		mib := []C.int{
			syscall.CTL_NET,
			syscall.AF_ROUTE,
			0,
			syscall.AF_INET,
			syscall.NET_RT_IFLIST2,
			C.int(m.Index),
		}
		if rv, err := C.sysctl(
			&mib[0],
			C.uint(len(mib)),
			unsafe.Pointer(nil),
			&size,
			unsafe.Pointer(nil),
			0,
		); rv != 0 {
			core.LogError(fmt.Errorf("IF info %v", err))
			continue
		}

		buf := make([]byte, size)
		i := (*C.struct_if_msghdr2)(unsafe.Pointer(&buf[0]))

		if rv, err := C.sysctl(
			&mib[0],
			6,
			unsafe.Pointer(i),
			&size,
			unsafe.Pointer(nil),
			0,
		); rv != 0 {
			core.LogError(fmt.Errorf("IF info %v", err))
			continue
		}
		if i.ifm_type != syscall.RTM_IFINFO2 {
			continue
		}

		m.Metrics = Metrics{
			Receive:            int(i.ifm_data.ifi_ibytes),
			ReceivePackets:     int(i.ifm_data.ifi_ipackets),
			ReceiveErrors:      int(i.ifm_data.ifi_ierrors),
			ReceiveDropped:     int(i.ifm_data.ifi_iqdrops),
			ReceiveMulticast:   int(i.ifm_data.ifi_imcasts),
			Transmit:           int(i.ifm_data.ifi_obytes),
			TransmitPackets:    int(i.ifm_data.ifi_opackets),
			TransmitErrors:     int(i.ifm_data.ifi_oerrors),
			TransmitDropped:    int(i.ifm_snd_drops),
			TransmitCollisions: int(i.ifm_data.ifi_collisions),
			TransmitMulticast:  int(i.ifm_data.ifi_omcasts),
		}

		ms = append(ms, m)
	}

	return
}
