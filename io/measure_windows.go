// Copyright © 2021-2023 The Gomon Project.

package io

import (
	"runtime"
	"unsafe"

	"github.com/StackExchange/wmi"
	"github.com/zosmac/gocore"
	"github.com/zosmac/gomon/message"
	"golang.org/x/sys/windows"
)

type (

	// Win32_LogicalDisk is a WMI Class for local storage device information.
	// The name of a WMI query response object must be identical to the name of a WMI Class,
	// the field names for the query. Go reflection is used to generate the query by the wmi package.
	// See https://docs.microsoft.com/en-us/windows/win32/cimwin32prov/win32-logicaldisk
	win32LogicalDisk struct {
		Name      string
		DriveType *uint32
		Size      *uint64
		BlockSize *uint64
	}

	ntfsStatistics struct {
		LogFileFullExceptions uint32
		OtherExceptions       uint32
		MftReads              uint32
		MftReadBytes          uint32
		MftWrites             uint32
		MftWriteBytes         uint32
		MftWritesUserLevel    struct {
			Write   uint16
			Create  uint16
			SetInfo uint16
			Flush   uint16
		}
		MftWritesFlushForLogFileFull uint16
		MftWritesLazyWriter          uint16
		MftWritesUserRequest         uint16
		Mft2Writes                   uint32
		Mft2WriteBytes               uint32
		Mft2WritesUserLevel          struct {
			Write   uint16
			Create  uint16
			SetInfo uint16
			Flush   uint16
		}
		Mft2WritesFlushForLogFileFull   uint16
		Mft2WritesLazyWriter            uint16
		Mft2WritesUserRequest           uint16
		RootIndexReads                  uint32
		RootIndexReadBytes              uint32
		RootIndexWrites                 uint32
		RootIndexWriteBytes             uint32
		BitmapReads                     uint32
		BitmapReadBytes                 uint32
		BitmapWrites                    uint32
		BitmapWriteBytes                uint32
		BitmapWritesFlushForLogFileFull uint16
		BitmapWritesLazyWriter          uint16
		BitmapWritesUserRequest         uint16
		BitmapWritesUserLevel           struct {
			Write   uint16
			Create  uint16
			SetInfo uint16
		}
		MftBitmapReads                     uint32
		MftBitmapReadBytes                 uint32
		MftBitmapWrites                    uint32
		MftBitmapWriteBytes                uint32
		MftBitmapWritesFlushForLogFileFull uint16
		MftBitmapWritesLazyWriter          uint16
		MftBitmapWritesUserRequest         uint16
		MftBitmapWritesUserLevel           struct {
			Write   uint16
			Create  uint16
			SetInfo uint16
			Flush   uint16
		}
		UserIndexReads      uint32
		UserIndexReadBytes  uint32
		UserIndexWrites     uint32
		UserIndexWriteBytes uint32
		LogFileReads        uint32
		LogFileReadBytes    uint32
		LogFileWrites       uint32
		LogFileWriteBytes   uint32
		Allocate            struct {
			Calls             uint32
			Clusters          uint32
			Hints             uint32
			RunsReturned      uint32
			HintsHonored      uint32
			HintsClusters     uint32
			Cache             uint32
			CacheClusters     uint32
			CacheMiss         uint32
			CacheMissClusters uint32
		}
		DiskResourcesExhausted uint32
	}

	filesystemStatistics struct {
		FileSystemType          uint16
		Version                 uint16
		SizeOfCompleteStructure uint32
		UserFileReads           uint32
		UserFileReadBytes       uint32
		UserDiskReads           uint32
		UserFileWrites          uint32
		UserFileWriteBytes      uint32
		UserDiskWrites          uint32
		MetaDataReads           uint32
		MetaDataReadBytes       uint32
		MetaDataDiskReads       uint32
		MetaDataWrites          uint32
		MetaDataWriteBytes      int32
		MetaDataDiskWrites      uint32
		ntfsStatistics
	}
)

// Measure captures system's I/O metrics.
func Measure() (ms []message.Content) {
	var path [windows.MAX_PATH + 1]uint16
	var fsType [windows.MAX_PATH + 1]uint16
	var target [windows.MAX_PATH + 1]uint16

	wos := []win32LogicalDisk{}
	if err := wmi.Query(wmi.CreateQuery(&wos, ""), &wos); err != nil {
		gocore.LogError("CreateQuery", err)
		return
	}

	for _, w := range wos {
		if *w.DriveType != windows.DRIVE_FIXED {
			continue
		}

		path[0], fsType[0], target[0] = 0, 0, 0 // null terminate
		volume, _ := windows.UTF16FromString(w.Name + "\\")
		windows.GetVolumeInformation(
			&volume[0],
			&path[0],
			uint32(len(path)-1),
			nil,
			nil,
			nil,
			&fsType[0],
			uint32(len(fsType)-1),
		)
		if windows.UTF16ToString(fsType[:]) != "NTFS" {
			continue
		}

		drive, _ := windows.UTF16FromString(w.Name)
		l, _ := windows.QueryDosDevice(
			&drive[0],
			&target[0],
			uint32(len(target)-1),
		)
		// strip \Device, prepend \\. to convert from ntfs to win32 namespace
		device := append([]uint16{'\\', '\\', '.'}, target[7:l]...)
		h, err := windows.CreateFile(
			&device[0],
			windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err != nil {
			gocore.LogError("CreateFile", err)
			continue
		}

		// https://msdn.microsoft.com/en-us/library/windows/desktop/aa364565(v=vs.85).aspx
		var stats *filesystemStatistics
		sizeof := (int(unsafe.Sizeof(*stats)) + 63) / 64 * 64 // round up by 64
		buf := make([]byte, sizeof*runtime.NumCPU())
		var size uint32
		err = windows.DeviceIoControl(
			h,
			0x00090060, // FSCTL_FILESYSTEM_GET_STATISTICS,
			nil,
			0,
			&buf[0],
			uint32(len(buf)),
			&size,
			nil,
		)
		if err != nil {
			gocore.LogError("DeviceIoControl", err)
			return
		}

		var reads, readBytes, writes, writeBytes uint64
		for i := 0; i < runtime.NumCPU(); i++ {
			stats := (*filesystemStatistics)(unsafe.Pointer(&buf[i*sizeof]))
			reads += uint64(stats.UserFileReads)
			readBytes += uint64(stats.UserFileReadBytes)
			writes += uint64(stats.UserFileWrites)
			writeBytes += uint64(stats.UserFileWriteBytes)
		}

		ms = append(ms, &measurement{
			Header: message.Measurement(),
			Id: Id{
				Device: windows.UTF16ToString(target[:l]),
			},
			Properties: Properties{
				Drive:          w.Name,
				DriveType:      gocore.DriveTypes[*w.DriveType],
				Path:           windows.UTF16ToString(path[:]),
				FilesystemType: windows.UTF16ToString(fsType[:]),
				TotalSize:      int(*wos[0].Size),
				BlockSize:      int(*wos[0].BlockSize),
			},
			Metrics: Metrics{
				ReadOperations:  int(reads),
				Read:            int(readBytes),
				WriteOperations: int(writes),
				Write:           int(writeBytes),
			},
		})
	}

	return
}
