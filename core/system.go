// Copyright © 2021 The Gomon Project.

package core

import (
	"os/user"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	// Boottime contains the system boottime.
	Boottime = boottime()

	// users caches user names for uids.
	users = names{
		lookup: func(id int) string {
			name := strconv.Itoa(id)
			if u, err := user.LookupId(name); err == nil {
				name = u.Name
			}
			return name
		},
		names: map[int]string{},
	}

	// groups caches group names for gids.
	groups = names{
		lookup: func(id int) string {
			name := strconv.Itoa(id)
			if g, err := user.LookupGroupId(name); err == nil {
				name = g.Name
			}
			return name
		},
		names: map[int]string{},
	}
)

type (
	// names defines a cache type for mapping ids to names.
	names struct {
		sync.RWMutex
		lookup func(int) string
		names  map[int]string
	}
)

// lookup retrieves and caches name for id.
func (ns *names) name(id int) string {
	ns.RLock()
	name, ok := ns.names[id]
	ns.RUnlock()
	if !ok {
		name = ns.lookup(id)
		ns.Lock()
		ns.names[id] = name
		ns.Unlock()
	}
	return name
}

// MsToTime converts Unix era milliseconds to Go time.Time.
func MsToTime(ms uint64) time.Time {
	var s, n int64
	if runtime.GOOS == "windows" {
		t := ms * 1e6                  // ns since 1/1/1601 overflows int64, use uint64
		s = int64(t/1e9 - 11644473600) // 1/1/1970 - 1/1/1601 offset in seconds
		n = int64(t % 1e9)
	} else {
		s = int64(ms / 1e3)       // truncate ms to s
		n = int64(ms % 1e3 * 1e6) // compute ns remainder
	}

	return time.Unix(s, n)
}

// Username retrieves and caches user name for uid.
func Username(uid int) string {
	return users.name(uid)
}

// Groupname retrieves and caches group name for gid.
func Groupname(gid int) string {
	return groups.name(gid)
}
