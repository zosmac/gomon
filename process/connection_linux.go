// Copyright © 2021 The Gomon Project.

package process

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/log"
)

var (
	// udpCache: UDP connections are not persistent, cache remote address and process to avoid repeated netlink queries.
	// However, we need a way to expire these as over time ports are reused. For now, just expire every 10 minutes.
	// udpCache = struct {
	// 	sync.Mutex
	// 	addresses map[string]string // map remote address to process
	// }{
	// 	addresses: map[string]string{},
	// }

	// netInodes built from /proc/net tcp, udp, and unix files
	netInodes map[int]netInode
)

type (
	// netInode represents a record from a /proc/net tcp, udp, or unix file.
	netInode struct {
		laddr net.Addr
		faddr net.Addr
	}
)

func init() {
	// go func() {
	// 	ticker := time.NewTicker(10 * time.Minute)
	// 	for range ticker.C {
	// 		udpCache.Lock()
	// 		udpCache.addresses = map[string]string{}
	// 		udpCache.Unlock()
	// 	}
	// }()
}

// endpoints returns a list of Connection structures for a process.
func (pid Pid) endpoints() Connections {
	dirname := filepath.Join("/proc", strconv.Itoa(int(pid)), "fd")
	dir, err := os.Open(dirname)
	if err != nil {
		return nil
	}
	fds, err := dir.Readdirnames(0)
	dir.Close()
	if err != nil {
		return nil
	}

	connm := Connections{}
	var names []string
	for _, fd := range fds {
		conn := fdConn(pid, fd)
		connm[conn.Descriptor] = conn
		if conn.Type == "REG" && conn.Name != "" && pid != Pid(os.Getpid()) {
			continue
		}
		names = append(names, conn.Name)
	}
	log.Watch(names, int(pid))

	return connm
}

// fdConn determines the connection type for a given file descriptor, given
// a file descriptor file name from /proc/pid/fd/.
func fdConn(pid Pid, sfd string) Connection {
	fd, _ := strconv.Atoi(sfd)
	lname := filepath.Join("/proc", strconv.Itoa(int(pid)), "fd", sfd)
	linfo, err := os.Lstat(lname)
	if err != nil {
		core.LogError(fmt.Errorf("lstat %s: %v", lname, err))
		return Connection{
			Descriptor: fd,
			Type:       "UNKNOWN",
		}
	}
	info, err := os.Stat(lname)
	if err != nil {
		core.LogError(fmt.Errorf("stat %s: %v", lname, err))
		return Connection{
			Descriptor: fd,
			Type:       "UNKNOWN",
		}
	}

	name, _ := os.Readlink(lname)
	conn := Connection{
		Descriptor: fd,
		Name:       name,
		Direction:  accmode(linfo.Mode().Perm()),
		inode:      int(info.Sys().(*syscall.Stat_t).Ino),
	}

	switch info.Mode() & os.ModeType {
	case os.ModeDevice:
		conn.Type = "BLK"
	case os.ModeDevice | os.ModeCharDevice:
		conn.Type = "CHR"
	case os.ModeDir:
		conn.Type = "DIR"
	case 0:
		conn.Type = "REG"
	case os.ModeSymlink:
		conn.Type = "LINK"
	case os.ModeNamedPipe:
		conn.Type = "FIFO"
		inode := strings.Trim(strings.TrimPrefix(name, "pipe:"), "[]")
		i, _ := strconv.Atoi(inode)
		conn.inode = i
		if conn.Direction == "-->>" {
			conn.Peer = inode
		} else {
			conn.Self = inode
		}
	case os.ModeSocket:
		inode := strings.Trim(strings.TrimPrefix(name, "socket:"), "[]")
		i, _ := strconv.Atoi(inode)
		conn.inode = i
		if ni, ok := netInodes[i]; ok {
			switch ni.laddr.(type) {
			case *net.TCPAddr, *net.UDPAddr:
				conn.Type = strings.ToUpper(ni.laddr.Network())
				conn.Self = ni.laddr.String()
				if ni.faddr == nil {
					conn.Peer = ""
					conn.Name = conn.Self + " (listen)"
				} else {
					conn.Peer = ni.faddr.String()
					conn.Name = conn.Peer + " -> " + conn.Self
				}
			case *net.UnixAddr:
				conn.Type = "UNIX"
				conn.Name = ni.laddr.String()
				conn.Self = inode
				if peer, err := nlUnixPeerInode(i); err == nil {
					conn.Peer = strconv.Itoa(peer)
				}
			}
		}
	}

	if conn.Name == os.DevNull {
		conn.Type = "NUL"
	}

	return conn
}

// accmode determines the I/O direction.
func accmode(mode os.FileMode) string {
	dir := "• • •"
	switch mode & (syscall.S_IREAD | syscall.S_IWRITE) {
	case syscall.S_IREAD | syscall.S_IWRITE:
		dir = "<-->"
	case syscall.S_IREAD:
		dir = "<<--"
	case syscall.S_IWRITE:
		dir = "-->>"
	}
	return dir
}

// getInodes reads the /proc/net tcp, udp, and unix files to build network connection inode table.
func getInodes() {
	inodes := map[int]netInode{}
	for _, p := range []string{"tcp", "tcp6", "udp", "udp6"} {
		f, _ := os.Open("/proc/net/" + p)
		sc := bufio.NewScanner(f)
		sc.Scan()
		sc.Text() // skip header
		for sc.Scan() {
			ni := netInode{}
			flds := strings.Split(sc.Text(), "\t")
			inode, _ := strconv.Atoi(flds[11])
			var lip, lp0, lp1 []byte
			var rip, rp0, rp1 []byte
			switch p {
			case "tcp", "udp":
				lip, _ = hex.DecodeString(flds[1][:8])
				lp0, _ = hex.DecodeString(flds[1][9:11])
				lp1, _ = hex.DecodeString(flds[1][11:13])
				rip, _ = hex.DecodeString(flds[2][:8])
				rp0, _ = hex.DecodeString(flds[2][9:11])
				rp1, _ = hex.DecodeString(flds[2][11:13])
			case "tcp6", "udp6":
				lip, _ = hex.DecodeString(flds[1][:32])
				lp0, _ = hex.DecodeString(flds[1][33:35])
				lp1, _ = hex.DecodeString(flds[1][35:37])
				rip, _ = hex.DecodeString(flds[2][:32])
				rp0, _ = hex.DecodeString(flds[2][33:35])
				rp1, _ = hex.DecodeString(flds[2][35:37])
			}
			lport := int(lp1[0])<<8 + int(lp0[0])
			fport := int(rp1[0])<<8 + int(rp0[0])
			switch p {
			case "tcp", "tcp6":
				ni.laddr = &net.TCPAddr{IP: net.IP(lip), Port: lport}
				if rp0[0] != 0 || rp1[0] != 0 {
					ni.faddr = &net.TCPAddr{IP: net.IP(rip), Port: fport}
				}
			case "udp", "udp6":
				ni.laddr = &net.UDPAddr{IP: net.IP(lip), Port: lport}
				if rp0[0] != 0 || rp1[0] != 0 {
					ni.faddr = &net.UDPAddr{IP: net.IP(rip), Port: fport}
				}
			}
			inodes[inode] = ni
		}
		f.Close()
	}
	f, _ := os.Open("/proc/net/unix")
	sc := bufio.NewScanner(f)
	sc.Scan()
	sc.Text() // skip header
	for sc.Scan() {
		flds := strings.Split(sc.Text(), "\t")
		inode, _ := strconv.Atoi(flds[6])
		var name string
		if len(flds) > 7 {
			name = flds[7]
		} else {
			name = "socketpair"
		}
		inodes[inode] = netInode{
			laddr: &net.UnixAddr{
				Name: name,
				Net:  "unix",
			},
		}
	}
	f.Close()

	netInodes = inodes
}
