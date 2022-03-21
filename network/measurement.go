// Copyright © 2021 The Gomon Project.

package network

import (
	"github.com/zosmac/gomon/message"
)

func init() {
	message.Document(&measurement{})
}

type (
	// Id identifies the message.
	Id struct {
		Name string `json:"name" gomon:"property"`
	}

	// Properties defines measurement properties.
	Properties struct {
		Index      int    `json:"index" gomon:"property"`
		Flags      string `json:"flags" gomon:"property"`
		Mtu        int    `json:"mtu" gomon:"property"`
		Mac        string `json:"mac" gomon:"property"`
		Address    string `json:"address" gomon:"property"`
		Netmask    string `json:"netmask,omitempty" gomon:"property"`
		Broadcast  string `json:"broadcast,omitempty" gomon:"property"`
		Linklocal6 string `json:"linklocal6,omitempty" gomon:"property"`
		Address6   string `json:"address6,omitempty" gomon:"property"`
	}

	// Metrics defines measurement metrics.
	Metrics struct {
		Receive            int `json:"receive" gomon:"counter,B"`
		ReceivePackets     int `json:"receive_packets" gomon:"counter,count"`
		ReceiveErrors      int `json:"receive_errors" gomon:"counter,count"`
		ReceiveDropped     int `json:"receive_dropped" gomon:"counter,count"`
		ReceiveOverruns    int `json:"receive_overruns" gomon:"counter,count,linux"`
		ReceiveFrame       int `json:"receive_frame" gomon:"counter,count,linux"`
		ReceiveCompressed  int `json:"receive_compressed" gomon:"counter,count,linux"`
		ReceiveMulticast   int `json:"receive_multicast" gomon:"counter,count"`
		Transmit           int `json:"transmit" gomon:"counter,B"`
		TransmitPackets    int `json:"transmit_packets" gomon:"counter,count"`
		TransmitErrors     int `json:"transmit_errors" gomon:"counter,count"`
		TransmitDropped    int `json:"transmit_dropped" gomon:"counter,count"`
		TransmitOverruns   int `json:"transmit_overruns" gomon:"counter,count,linux"`
		TransmitCollisions int `json:"transmit_collisions" gomon:"counter,count,!windows"`
		TransmitCarrier    int `json:"transmit_carrier" gomon:"counter,count,linux"`
		TransmitCompressed int `json:"transmit_compressed" gomon:"counter,count,linux"`
		TransmitMulticast  int `json:"transmit_multicast" gomon:"counter,count"`
	}

	// measurement for the message.
	measurement struct {
		message.Header `gomon:""`
		Id             `json:"id" gomon:""`
		Properties     `gomon:""`
		Metrics        `gomon:""`
	}
)

// Events returns the list of acceptable Event values for this message.
func (*measurement) Events() []string {
	return message.MeasureEvents.Values()
}

// ID returns the identifier for a network interface message.
func (m *measurement) ID() string {
	return m.Id.Name
}
