// Copyright © 2021 The Gomon Project.

package process

import (
	"time"

	"github.com/zosmac/gomon/message"
	"golang.org/x/sys/unix"
)

func init() {
	message.Document(&tsMeasurement{})
}

const (
	// message events.
	netlinkTaskstats netlinkEvent = "taskstats"
)

var (
	// netlinkEvents valid event values for messages.
	netlinkEvents = message.ValidValues{
		netlinkTaskstats,
	}
)

type (
	// netlinkEvent type.
	netlinkEvent string

	// Taskstats metrics reported on netlink
	Taskstats struct {
		message.Header        `gomon:""`
		Version               uint16        `json:"version" gomon:"property"`
		AcExitcode            uint32        `json:"ac_exitcode" gomon:"property"`
		AcFlag                uint8         `json:"ac_flag" gomon:"property"`
		AcNice                uint8         `json:"ac_nice" gomon:"property"`
		CPUCount              uint64        `json:"cpu_count" gomon:"gauge,count"`
		CPUDelayTotal         time.Duration `json:"cpu_delay_total" gomon:"counter,ns"`
		BlkioCount            uint64        `json:"blkio_count" gomon:"counter,count"`
		BlkioDelayTotal       time.Duration `json:"blkio_delay_total" gomon:"counter,ns"`
		SwapinCount           uint64        `json:"swapin_count" gomon:"counter,count"`
		SwapinDelayTotal      time.Duration `json:"swapin_delay_total" gomon:"counter,ns"`
		CPURunRealTotal       time.Duration `json:"cpu_run_real_total" gomon:"counter,ns"`
		CPURunVirtualTotal    time.Duration `json:"cpu_run_virtual_total" gomon:"counter,ns"`
		AcComm                [32]int8      `json:"ac_comm" gomon:"property"`
		AcSched               uint8         `json:"ac_sched" gomon:"property"`
		AcPad                 [3]uint8
		_                     [4]byte
		AcUID                 uint32        `json:"ac_uid" gomon:"property"`
		AcGID                 uint32        `json:"ac_gid" gomon:"property"`
		AcPid                 uint32        `json:"ac_pid" gomon:"property"`
		AcPpid                uint32        `json:"ac_ppid" gomon:"property"`
		AcBtime               uint32        `json:"ac_btime" gomon:"property"`
		AcEtime               time.Duration `json:"ac_etime" gomon:"counter,ns"`
		AcUtime               time.Duration `json:"ac_utime" gomon:"counter,ns"`
		AcStime               time.Duration `json:"ac_stime" gomon:"counter,ns"`
		AcMinflt              uint64        `json:"ac_minflt" gomon:"counter,count"`
		AcMajflt              uint64        `json:"ac_majflt" gomon:"counter,count"`
		Coremem               uint64        `json:"coremem" gomon:"gauge,MB/us"`
		Virtmem               uint64        `json:"virtmem" gomon:"gauge,MB/us"`
		HiwaterRss            uint64        `json:"hiwater_rss" gomon:"gauge,B"`
		HiwaterVM             uint64        `json:"hiwater_vm" gomon:"gauge,B"`
		ReadChar              uint64        `json:"read_char" gomon:"counter,count"`
		WriteChar             uint64        `json:"write_char" gomon:"gauge,count"`
		ReadSyscalls          uint64        `json:"read_syscalls" gomon:"gauge,count"`
		WriteSyscalls         uint64        `json:"write_syscalls" gomon:"gauge,count"`
		Read                  uint64        `json:"read" gomon:"counter,B"`
		Write                 uint64        `json:"write" gomon:"counter,B"`
		CancelledWrite        uint64        `json:"cancelled_write" gomon:"counter,B"`
		Nvcsw                 uint64        `json:"nvcsw" gomon:"counter,count"`
		Nivcsw                uint64        `json:"nivcsw" gomon:"counter,count"`
		AcUtimescaled         time.Duration `json:"ac_utimescaled" gomon:"counter,ns"`
		AcStimescaled         time.Duration `json:"ac_stimescaled" gomon:"counter,ns"`
		CPUScaledRunRealTotal time.Duration `json:"cpu_scaled_run_real_total" gomon:"counter,ns"`
		FreepagesCount        uint64        `json:"freepages_count" gomon:"counter,count"`
		FreepagesDelayTotal   time.Duration `json:"freepages_delay_total" gomon:"counter,ns"`
		ThrashingCount        uint64        `json:"thrashing_count" gomon:"counter,count"`
		ThrashingDelayTotal   time.Duration `json:"thrashing_delay_total" gomon:"counter,ns"`
		AcBtime64             uint64        `json:"ac_btime64" gomon:"property"`
		CompactCount          uint64        `json:"compact_count" gomon:"counter,count"`
		CompactDelayTotal     time.Duration `json:"compact_delay_total" gomon:"counter,ns"`
		AcTgid                uint32        `json:"ac_tgid" gomon:"property"`
		AcTgetime             time.Duration `json:"ac_tgettime" gomon:"counter,ns"`
		AcExeDev              uint64        `json:"ac_exe_dev" gomon:"counter,count"`
		AcExeInode            uint64        `json:"ac_exe_inode" gomon:"counter,count"`
		WpcopyCount           uint64        `json:"wpcopy_count" gomon:"counter,count"`
		WpcopyDelayTotal      uint64        `json:"wpcopy_delay_total" gomon:"counter,ns"`
	}

	// measurement for the message.
	tsMeasurement struct {
		message.Header `gomon:""`
		Id             `json:"id" gomon:""`
		unix.Taskstats `gomon:""`
		// Go formatted taskstats fields
		Uname string `json:"uname" gomon:"property"`
		Gname string `json:"gname" gomon:"property"`
	}
)

// String returns the event value of the message as a string.
func (ev netlinkEvent) String() string {
	return string(ev)
}

// ValidValues returns the valid event values for the message.
func (netlinkEvent) ValidValues() message.ValidValues {
	return netlinkEvents
}

// Events returns the list of acceptable Event values for this message.
func (*tsMeasurement) Events() []string {
	return netlinkEvents.Values()
}

// ID returns the identifier for a process message.
func (m *tsMeasurement) ID() string {
	return m.Id.Name + "[" + m.Id.Pid.String() + "]"
}
