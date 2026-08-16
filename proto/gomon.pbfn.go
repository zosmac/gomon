package proto

import (
	"github.com/zosmac/gomon/file"
	"github.com/zosmac/gomon/filesystem"
	"github.com/zosmac/gomon/io"
	"github.com/zosmac/gomon/logs"
	"github.com/zosmac/gomon/network"
	"github.com/zosmac/gomon/process"
	"github.com/zosmac/gomon/serve"
	"github.com/zosmac/gomon/system"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func toInt64(n int) *int64 {
	i := int64(n)
	return &i
}

func CopyFileObservation(src *file.Observation) *FileObservation {
	b := FileObservation_builder{
		Timestamp:   timestamppb.New(src.Timestamp),
		Host:        &src.Host,
		Platform:    &src.Platform,
		Source:      &src.Source,
		Event:       (*string)(&src.Event),
		Name:        &src.Name,
		FileEventId: &src.FileEventId,
		Message:     &src.Message,
	}
	return b.Build()
}

func CopyFilesystemMeasurement(src *filesystem.Measurement) *FilesystemMeasurement {
	b := FilesystemMeasurement_builder{
		Timestamp: timestamppb.New(src.Timestamp),
		Host:      &src.Host,
		Platform:  &src.Platform,
		Source:    &src.Source,
		Event:     (*string)(&src.Event),
		Mount:     &src.Mount,
		Path:      &src.Path,
		Type:      &src.Type,
		Total:     toInt64(int(src.Total)),
		Used:      toInt64(int(src.Used)),
		Free:      toInt64(int(src.Free)),
		Available: toInt64(int(src.Available)),
		Files:     toInt64(int(src.Files)),
		FreeFiles: toInt64(int(src.FreeFiles)),
	}
	return b.Build()
}

func CopyIoMeasurement(src *io.Measurement) *IoMeasurement {
	b := IoMeasurement_builder{
		Timestamp:       timestamppb.New(src.Timestamp),
		Host:            &src.Host,
		Platform:        &src.Platform,
		Source:          &src.Source,
		Event:           (*string)(&src.Event),
		Device:          &src.Device,
		Major:           &src.Major,
		Minor:           &src.Minor,
		TotalSize:       toInt64(int(src.TotalSize)),
		BlockSize:       toInt64(int(src.BlockSize)),
		ReadOperations:  toInt64(int(src.ReadOperations)),
		Read:            toInt64(int(src.Read)),
		ReadTime:        durationpb.New(src.ReadTime),
		WriteOperations: toInt64(int(src.WriteOperations)),
		Write:           toInt64(int(src.Write)),
		WriteTime:       durationpb.New(src.WriteTime),
	}
	return b.Build()
}

func CopyLogsObservation(src *logs.Observation) *LogsObservation {
	b := LogsObservation_builder{
		Timestamp: timestamppb.New(src.Timestamp),
		Host:      &src.Host,
		Platform:  &src.Platform,
		Source:    &src.Source,
		Event:     (*string)(&src.Event),
		Name:      &src.Name,
		Pid:       toInt64(int(src.Pid)),
		Sender:    &src.Sender,
		Message:   &src.Message,
	}
	return b.Build()
}

func CopyNetworkMeasurement(src *network.Measurement) *NetworkMeasurement {
	b := NetworkMeasurement_builder{
		Timestamp:          timestamppb.New(src.Timestamp),
		Host:               &src.Host,
		Platform:           &src.Platform,
		Source:             &src.Source,
		Event:              (*string)(&src.Event),
		Name:               &src.Name,
		Index:              toInt64(int(src.Index)),
		Flags:              &src.Flags,
		Mtu:                toInt64(int(src.Mtu)),
		Mac:                &src.Mac,
		Address:            &src.Address,
		Netmask:            &src.Netmask,
		Broadcast:          &src.Broadcast,
		Linklocal6:         &src.Linklocal6,
		Address6:           &src.Address6,
		Receive:            toInt64(int(src.Receive)),
		ReceivePackets:     toInt64(int(src.ReceivePackets)),
		ReceiveErrors:      toInt64(int(src.ReceiveErrors)),
		ReceiveDropped:     toInt64(int(src.ReceiveDropped)),
		ReceiveMulticast:   toInt64(int(src.ReceiveMulticast)),
		Transmit:           toInt64(int(src.Transmit)),
		TransmitPackets:    toInt64(int(src.TransmitPackets)),
		TransmitErrors:     toInt64(int(src.TransmitErrors)),
		TransmitDropped:    toInt64(int(src.TransmitDropped)),
		TransmitCollisions: toInt64(int(src.TransmitCollisions)),
		TransmitMulticast:  toInt64(int(src.TransmitMulticast)),
	}
	return b.Build()
}

func CopyProcessObservation(src *process.Observation) *ProcessObservation {
	b := ProcessObservation_builder{
		Timestamp: timestamppb.New(src.Timestamp),
		Host:      &src.Host,
		Platform:  &src.Platform,
		Source:    &src.Source,
		Event:     (*string)(&src.Event),
		Name:      &src.Name,
		Pid:       toInt64(int(src.Pid)),
		Starttime: timestamppb.New(src.Starttime),
		Message:   &src.Message,
	}
	return b.Build()
}

func CopyProcessMeasurement(src *process.Measurement) *ProcessMeasurement {
	b := ProcessMeasurement_builder{
		Timestamp:  timestamppb.New(src.Timestamp),
		Host:       &src.Host,
		Platform:   &src.Platform,
		Source:     &src.Source,
		Event:      (*string)(&src.Event),
		Name:       &src.Name,
		Pid:        toInt64(int(src.Pid)),
		Starttime:  timestamppb.New(src.Starttime),
		Ppid:       toInt64(int(src.Ppid)),
		Pgid:       toInt64(int(src.Pgid)),
		Tty:        &src.Tty,
		Uid:        toInt64(int(src.Uid)),
		Gid:        toInt64(int(src.Gid)),
		Username:   &src.Username,
		Groupname:  &src.Groupname,
		Executable: &src.Executable,
		Args:       src.Args,
		Envs:       src.Envs,
		Cwd:        &src.Cwd,
		Root:       &src.Root,
		Connections: func() []*ProcessMeasurement_Connection {
			result := make([]*ProcessMeasurement_Connection, len(src.Connections))
			for i, v := range src.Connections {
				result[i] = ProcessMeasurement_Connection_builder{
					Type: &v.Type,
					Self: ProcessMeasurement_Connection_Endpoint_builder{
						Name: &v.Self.Name,
						Pid:  toInt64(int(v.Self.Pid)),
					}.Build(),
					Peer: ProcessMeasurement_Connection_Endpoint_builder{
						Name: &v.Peer.Name,
						Pid:  toInt64(int(v.Peer.Pid)),
					}.Build(),
				}.Build()
			}
			return result
		}(),
		Status:          &src.Status,
		Nice:            toInt64(int(src.Nice)),
		Priority:        toInt64(int(src.Priority)),
		Threads:         toInt64(int(src.Threads)),
		User:            durationpb.New(src.User),
		System:          durationpb.New(src.System),
		Total:           durationpb.New(src.Total),
		Size:            toInt64(int(src.Size)),
		Resident:        toInt64(int(src.Resident)),
		PageFaults:      toInt64(int(src.PageFaults)),
		ContextSwitches: toInt64(int(src.ContextSwitches)),
		ReadActual:      toInt64(int(src.ReadActual)),
		WriteActual:     toInt64(int(src.WriteActual)),
		WriteRequested:  toInt64(int(src.WriteRequested)),
	}
	return b.Build()
}

func CopyServeMeasurement(src *serve.Measurement) *ServeMeasurement {
	b := ServeMeasurement_builder{
		Timestamp:      timestamppb.New(src.Timestamp),
		Host:           &src.Host,
		Platform:       &src.Platform,
		Source:         &src.Source,
		Event:          (*string)(&src.Event),
		Name:           &src.Name,
		Address:        &src.Address,
		Endpoints:      src.Endpoints,
		HttpRequests:   toInt64(int(src.HttpRequests)),
		Collections:    toInt64(int(src.Collections)),
		CollectionTime: durationpb.New(src.CollectionTime),
		LokiStreams:    toInt64(int(src.LokiStreams)),
	}
	return b.Build()
}

func CopySystemMeasurement(src *system.Measurement) *SystemMeasurement {
	b := SystemMeasurement_builder{
		Timestamp:           timestamppb.New(src.Timestamp),
		Host:                &src.Host,
		Platform:            &src.Platform,
		Source:              &src.Source,
		Event:               (*string)(&src.Event),
		Name:                &src.Name,
		Boottime:            timestamppb.New(src.Boottime),
		Uptime:              durationpb.New(src.Uptime),
		ProcessesMaximum:    toInt64(int(src.ProcessesMaximum)),
		ProcessesPerUser:    toInt64(int(src.ProcessesPerUser)),
		OpenFilesMaximum:    toInt64(int(src.OpenFilesMaximum)),
		OpenFilesPerProcess: toInt64(int(src.OpenFilesPerProcess)),
		OpenFilesCurrent:    toInt64(int(src.OpenFilesCurrent)),
		LoadAverage: SystemMeasurement_LoadAverage_builder{
			OneMinute:     &src.LoadAverage.OneMinute,
			FiveMinute:    &src.LoadAverage.FiveMinute,
			FifteenMinute: &src.LoadAverage.FifteenMinute,
		}.Build(),
		Cpu: SystemMeasurement_Cpu_builder{
			Total:  durationpb.New(src.Cpu.Total),
			User:   durationpb.New(src.Cpu.User),
			System: durationpb.New(src.Cpu.System),
			Idle:   durationpb.New(src.Cpu.Idle),
		}.Build(),
		CpuCount: toInt64(int(src.CpuCount)),
		Cpus: func() []*SystemMeasurement_Cpu {
			result := make([]*SystemMeasurement_Cpu, len(src.Cpus))
			for i, v := range src.Cpus {
				result[i] = SystemMeasurement_Cpu_builder{
					Total:  durationpb.New(v.Total),
					User:   durationpb.New(v.User),
					System: durationpb.New(v.System),
					Idle:   durationpb.New(v.Idle),
				}.Build()
			}
			return result
		}(),
		Memory: SystemMeasurement_Memory_builder{
			Total:      toInt64(int(src.Memory.Total)),
			Free:       toInt64(int(src.Memory.Free)),
			Used:       toInt64(int(src.Memory.Used)),
			FreeActual: toInt64(int(src.Memory.FreeActual)),
			UsedActual: toInt64(int(src.Memory.UsedActual)),
		}.Build(),
		Swap: SystemMeasurement_Swap_builder{
			Total: toInt64(int(src.Swap.Total)),
			Free:  toInt64(int(src.Swap.Free)),
			Used:  toInt64(int(src.Swap.Used)),
		}.Build(),
		ProcessStats: SystemMeasurement_ProcStats_builder{
			Count:  toInt64(int(src.ProcessStats.Count)),
			Active: toInt64(int(src.ProcessStats.Active)),
			Execed: toInt64(int(src.ProcessStats.Execed)),
			Exited: toInt64(int(src.ProcessStats.Exited)),
			Cpu:    durationpb.New(src.ProcessStats.Cpu),
		}.Build(),
	}
	return b.Build()
}
