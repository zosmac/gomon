// Copyright © 2021-2023 The Gomon Project.

package network

import (
	"github.com/zosmac/gomon/message"
	"golang.org/x/sys/windows"
)

// Measure captures system's network interface metrics.
func Measure() (ms []message.Content) {
	for _, m := range interfaces() {
		var mibIfRow windows.MibIfRow
		mibIfRow.Index = uint32(m.Index)

		if windows.GetIfEntry(&mibIfRow) != nil {
			continue
		}

		m.Metrics = Metrics{
			Receive:           int(mibIfRow.InOctets),
			ReceivePackets:    int(mibIfRow.InUcastPkts),
			ReceiveErrors:     int(mibIfRow.InErrors),
			ReceiveDropped:    int(mibIfRow.InDiscards + mibIfRow.InUnknownProtos),
			ReceiveMulticast:  int(mibIfRow.InNUcastPkts),
			Transmit:          int(mibIfRow.OutOctets),
			TransmitPackets:   int(mibIfRow.OutUcastPkts),
			TransmitErrors:    int(mibIfRow.OutErrors),
			TransmitDropped:   int(mibIfRow.OutDiscards),
			TransmitMulticast: int(mibIfRow.OutNUcastPkts),
		}
		ms = append(ms, m)
	}

	return
}
