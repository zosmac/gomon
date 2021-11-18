// Copyright © 2021 The Gomon Project.

package network

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/zosmac/gomon/message"
)

// Measure captures system's network interface metrics.
func Measure() (ms []message.Content) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return
	}
	defer f.Close()

	nums := make([]int, 16)
	numa := make([]interface{}, 16)
	for i := range numa {
		numa[i] = &nums[i]
	}

	is := interfaces()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		s := strings.Split(sc.Text(), ":")
		if len(s) == 2 {
			name := strings.TrimSpace(s[0])
			fmt.Sscanf(s[1], "%d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d", numa...)
			for _, m := range is {
				if name == m.Id.Name {
					m.Metrics = Metrics{
						Receive:            nums[0],
						ReceivePackets:     nums[1],
						ReceiveErrors:      nums[2],
						ReceiveDropped:     nums[3],
						ReceiveOverruns:    nums[4],
						ReceiveFrame:       nums[5],
						ReceiveCompressed:  nums[6],
						ReceiveMulticast:   nums[7],
						Transmit:           nums[8],
						TransmitPackets:    nums[9],
						TransmitErrors:     nums[10],
						TransmitDropped:    nums[11],
						TransmitOverruns:   nums[12],
						TransmitCollisions: nums[13],
						TransmitCarrier:    nums[14],
						TransmitCompressed: nums[15],
					}
					ms = append(ms, m)
					break
				}
			}
		}
	}

	return ms
}
