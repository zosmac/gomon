// Copyright © 2021 The Gomon Project.

package main

/*
#cgo CFLAGS: -x objective-c -std=gnu11 -fobjc-arc -D__unix__
#cgo LDFLAGS: -framework CoreFoundation -framework AppKit -framework Foundation
#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>

static CFTypeRef
Observer() {
	NSNotificationCenter *center = [NSNotificationCenter defaultCenter];
	NSLog(@"Center is %@", center);
	NSOperationQueue *queue = [NSOperationQueue mainQueue];
	NSLog(@"Queue is %@", queue);
	fprintf(stderr, "===== here I am!!!\n");
	NSObject *observer = [center addObserverForName: @"AppleInterfaceThemeChangedNotification"
	// NSObject *observer = [center addObserverForName: NSSystemColorsDidChangeNotification
	// NSObject *observer = [center addObserverForName: NSApplicationDidFinishLaunchingNotification
		object: nil
		queue: queue
		usingBlock: ^(NSNotification *note) {
			NSLog(@"Note is %@", note);
			fprintf(stderr, "===== notification!!!!!!\n");
		}
	];
	fprintf(stderr, "===== observer added!!!!!!\n");

	return (__bridge_retained CFTypeRef)observer;
}

@interface MyView : NSView
@end
@implementation MyView
- (id)initWithFrame:(CGRect)frame
{
    self = [super initWithFrame:frame];
    return self;
}
- (void)viewDidChangeEffectiveAppearance {
	NSString *name = [[self effectiveAppearance] name];
	NSLog(@"Changed Appearance is %@", name);
}
- (void)drawRect:(NSRect)rect
{
    // erase the background by drawing white
    [[NSColor whiteColor] set];
    [NSBezierPath fillRect:rect];
}
@end

static CFTypeRef
View() {
	NSRect rect = NSMakeRect(100.0, 100.0, 100.0, 100.0);
	MyView *view = [[MyView alloc] initWithFrame: rect];
	return (__bridge_retained CFStringRef)view;
}

static CFTypeRef
Window() {
	[[NSOperationQueue mainQueue] addOperationWithBlock: ^() {

	[NSApplication sharedApplication];
	NSRect rect = NSMakeRect(100.0, 100.0, 100.0, 100.0);
	NSWindow *window = [[NSWindow alloc]
		initWithContentRect:rect
                  styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable
                    backing:NSBackingStoreBuffered
                      defer:NO
		];
	}];
	return nil; // (__bridge_retained CFStringRef)window;
}

static void
Run() {
	[NSApp run];
}

static BOOL
DarkMode(CFTypeRef w) {
	[[NSOperationQueue mainQueue] addOperationWithBlock: ^() {

	NSArray *names = @[NSAppearanceNameAqua, NSAppearanceNameDarkAqua];
	// MyView *view = (__bridge MyView *)v;
	NSWindow *window = [NSApp mainWindow]; // (__bridge NSWindow *)w;
	NSView *view = [window contentView];
	// [view setNeedsDisplay: TRUE];
	// NSOperationQueue *queue = [NSOperationQueue mainQueue];
	// [queue addBarrierBlock: ^() {
	// 		NSString *name = [[view effectiveAppearance] bestMatchFromAppearancesWithNames: names];
	// 		NSLog(@"Appearance is %@", name);
	// 	}
	// ];
	NSString *name = [[view effectiveAppearance] bestMatchFromAppearancesWithNames: names];
	NSLog(@"Appearance is %@", name);
	}];

	return TRUE; // (name == NSAppearanceNameDarkAqua) ? TRUE : FALSE;

	// NSString *osxMode = [[NSUserDefaults standardUserDefaults] stringForKey:@"AppleInterfaceStyle"];
	// if (osxMode == nil) {
	// 	osxMode = @"Bright";
	// }
	// return (__bridge_retained CFStringRef)osxMode;
}
*/
import "C"

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/process"
)

var (
	// hnMap caches resolver host name lookup.
	hnMap  = map[string]string{}
	hnLock sync.Mutex

	window C.CFTypeRef
	view   C.CFTypeRef

	// graphviz color attributes for nodes, edges
	fgColor string
	bgColor string
	colors  []string
)

const (
	// graphviz cluster attributes
	cluster = `fontsize=11 penwidth=3.0 pencolor="#4488CC"`
)

type (
	// Pid alias for Pid in process package.
	Pid = process.Pid

	// query from http request.
	query struct {
		pid Pid
	}
)

func init() {
	// obs := C.Observer()
	// fmt.Fprintf(os.Stderr, "observer is %v\n", obs)
	// view = C.View()
	go func() {
		window = C.Window()
		C.Run()
	}()
}

func setMode() {
	if C.DarkMode(window) {
		fgColor = "#FFFFFF"
		bgColor = "#000000"
		colors = []string{"#FFFF33", "#FF33FF", "#33FFFF", "#FFCC66", "#CC66FF", "#66FFCC", "#CCCC99", "#CC99CC", "#99CCCC", "#FF66CC", "#66CC11", "#CCFF66"}
	} else {
		fgColor = "#000000"
		bgColor = "#FFFFFF"
		colors = []string{"#1111DD", "#11DD11", "#DD1111", "#1144AA", "#44AA11", "#AA1144", "#444477", "#447744", "#774444", "#11AA44", "#AA4411", "#4411AA"}
	}
}

// color defines the color for graphviz nodes and edges
func color(pid Pid) string {
	color := fgColor
	if pid < 0 {
		pid = -pid
	}
	if pid > 0 {
		color = colors[(int(pid-1))%len(colors)]
	}
	return color
}

// NodeGraph produces the process connections node graph.
func NodeGraph(req *http.Request) []byte {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[:n]
			core.LogError(fmt.Errorf("NodeGraph() panicked, %v\n%s", r, buf))
		}
	}()

	var (
		clusterEdges string
		hosts        string
		hostNode     Pid
		hostEdges    string
		processes    []string
		processNodes []Pid
		processEdges []string
		datas        string
		dataNode     Pid
		dataEdges    string
		include      = map[Pid]struct{}{} // record which processes have a connection to include in report
		nodes        = map[Pid]struct{}{}
	)

	setMode()

	query, _ := parseQuery(req)

	ft := process.Table{}
	pt := process.BuildTable()
	process.Connections(pt)

	if query.pid != 0 && pt[query.pid] == nil {
		query.pid = 0 // reset to default
	}
	if query.pid > 0 { // build this process' "extended family"
		ft = family(pt, query.pid)
	} else { // only consider non-daemon and remote host connected processes
		for pid, p := range pt {
			if p.Ppid > 1 {
				for pid, p := range family(pt, pid) {
					ft[pid] = p
				}
			}
			for _, conn := range p.Connections {
				if conn.Peer.Pid < 0 {
					ft[conn.Self.Pid] = pt[conn.Self.Pid]
				}
			}
		}
	}

	em := map[string]string{}

	for _, p := range ft {
		for _, conn := range p.Connections {
			if conn.Self.Pid == 0 || conn.Peer.Pid == 0 || // ignore kernel process
				conn.Self.Pid == 1 || conn.Peer.Pid == 1 || // ignore launchd processes
				conn.Self.Pid == conn.Peer.Pid || // ignore inter-process connections
				query.pid == 0 && conn.Peer.Pid >= math.MaxInt32 { // ignore data connections for the "all process" query
				continue
			}

			include[conn.Self.Pid] = struct{}{}

			if conn.Peer.Pid < 0 { // peer is remote host or listener
				host, port, _ := net.SplitHostPort(conn.Peer.Name)

				dir := "forward"
				// name for listen port is device inode: on linux decimal and on darwin hexadecimal
				if _, err := strconv.Atoi(conn.Self.Name); err == nil || conn.Self.Name[0:2] == "0x" { // listen socket
					dir = "back"
				}

				if _, ok := nodes[conn.Peer.Pid]; !ok {
					nodes[conn.Peer.Pid] = struct{}{}
					hosts += fmt.Sprintf(`
    %d [shape=cds height=0.6 fillcolor=%q label="%s:%s\n%s"]`,
						conn.Peer.Pid,
						color(conn.Peer.Pid),
						conn.Type,
						port,
						hostname(host),
					)
				}
				if hostNode == 0 {
					hostNode = conn.Peer.Pid
				}

				// TODO: host arrow on east/right edge
				hostEdges += fmt.Sprintf(`
  %d -> %d [dir=%s color=%q tooltip="%s ‑> %s\n%s"]`, // non-breaking space/hyphen
					conn.Peer.Pid,
					conn.Self.Pid,
					dir,
					color(conn.Peer.Pid)+";0.5:"+color(conn.Self.Pid),
					conn.Type+":"+conn.Peer.Name,
					conn.Self.Name,
					shortname(pt, conn.Self.Pid),
				)
			} else if conn.Peer.Pid >= math.MaxInt32 { // peer is data
				peer := conn.Type + ":" + conn.Peer.Name

				datas += fmt.Sprintf(`
    %d [shape=note fillcolor=%q label=%q]`,
					conn.Peer.Pid,
					color(conn.Peer.Pid),
					peer,
				)
				if dataNode == 0 {
					dataNode = conn.Peer.Pid
				}

				// show edge for data connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				if _, ok := em[id]; !ok {
					em[id] = ""
					dataEdges += fmt.Sprintf(`
  %d -> %d [dir=forward color=%q tooltip="%s\n%s"]`,
						conn.Self.Pid,
						conn.Peer.Pid,
						color(conn.Self.Pid)+";0.5:"+color(conn.Peer.Pid),
						shortname(pt, conn.Self.Pid),
						peer,
					)
				}
			} else { // peer is process
				include[conn.Peer.Pid] = struct{}{}

				depth := len(pt[conn.Self.Pid].Ancestors)
				for i := len(processNodes); i <= depth; i++ {
					processNodes = append(processNodes, 0)
					processEdges = append(processEdges, "")
				}
				if processNodes[depth] == 0 {
					processNodes[depth] = conn.Self.Pid
				}

				if conn.Type == "parent" {
					processEdges[depth] += fmt.Sprintf(`
  %d -> %d [dir=forward tooltip="%s ‑> %s\n"]`, // non-breaking space/hyphen
						conn.Self.Pid,
						conn.Peer.Pid,
						shortname(pt, conn.Self.Pid),
						shortname(pt, conn.Peer.Pid),
					)
					continue
				}

				// show edge for inter-process connections only once
				id := fmt.Sprintf("%d -> %d", conn.Self.Pid, conn.Peer.Pid)
				di := fmt.Sprintf("%d -> %d", conn.Peer.Pid, conn.Self.Pid)

				_, ok := em[id]
				if ok {
					em[id] += fmt.Sprintf("%s:%s ‑> %s\n", // non-breaking space/hyphen
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
					)
				} else if _, ok = em[di]; ok {
					em[di] += fmt.Sprintf("%s:%s ‑> %s\n", // non-breaking space/hyphen
						conn.Type,
						conn.Peer.Name,
						conn.Self.Name,
					)
				} else {
					em[id] = fmt.Sprintf("%s ‑> %s\n%s:%s ‑> %s\n", // non-breaking space/hyphen
						shortname(pt, conn.Self.Pid),
						shortname(pt, conn.Peer.Pid),
						conn.Type,
						conn.Self.Name,
						conn.Peer.Name,
					)
				}
			}
		}
	}

	for pid, p := range pt {
		if _, ok := include[pid]; !ok {
			continue
		}

		for i := len(processes); i <= len(p.Ancestors); i++ {
			processes = append(processes, fmt.Sprintf(`
    subgraph processes_%d {
      cluster=true label="Process depth %[1]d" rank=same %s`,
				i+1,
				cluster))
		}

		node := fmt.Sprintf(`
      %d [shape=rect style="rounded,filled" fillcolor=%q URL="http://localhost:%d/gomon?pid=\N" label="%s\n\N" tooltip=%q]`,
			pid,
			color(pid),
			core.Flags.Port,
			pt[pid].Id.Name,
			longname(pt, pid),
		)
		processes[len(p.Ancestors)] += node

		depth := len(pt[pid].Ancestors)

		for edge, tooltip := range em {
			fields := strings.Fields(edge)
			self, _ := strconv.Atoi(fields[0])
			peer, _ := strconv.Atoi(fields[2])
			// if strings.Fields(edge)[0] == strconv.Itoa(int(pid)) {
			if Pid(self) == pid {
				if tooltip != "" {
					processEdges[depth] += fmt.Sprintf(`
  %s [dir=both color=%q tooltip=%q]`,
						edge,
						color(Pid(self))+";0.5:"+color(Pid(peer)),
						tooltip,
					)
				}
				delete(em, edge)
			}
		}
	}

	for i := range processes {
		processes[i] += "\n    }"
	}

	if len(processNodes) > 0 {
		if hostNode != 0 {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="hosts" lhead="processes_1"]`,
				hostNode,
				processNodes[0],
			)
		}
		for i := range processNodes[:len(processNodes)-1] {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="processes_%d" lhead="processes_%d"]`,
				processNodes[i],
				processNodes[i+1],
				i+1,
				i+2,
			)
		}
		if dataNode != 0 {
			clusterEdges += fmt.Sprintf(`
  %d -> %d [style=invis ltail="processes_%d" lhead="files"]`,
				processNodes[len(processNodes)-1],
				dataNode,
				len(processNodes),
			)
		}
	}

	label := fmt.Sprintf(`
<table href="http://localhost:%d/gomon?dark">
  <tr>
    <td align="left" colspan="2">
      External and Inter-Process Connections
    </td>
  </tr>
  <tr>
    <td align="left">
      Host: <b>%s</b>`, core.Flags.Port, core.Hostname)
	if query.pid > 0 {
		label += fmt.Sprintf(`
    </td>
    <td align="left">
      Process: <b>%s</b>`, shortname(pt, query.pid))
	}
	label += time.Now().Local().Format(`
    </td>
  </tr>
  <tr>
    <td align="left" colspan="2">
      Mon Jan 02 2006 at 03:04:05PM MST
    </td>
  </tr>
</table>`,
	)

	return dot(`digraph "gomon process nodes" {
  fontname=helvetica
  fontcolor="` + fgColor + `"
  bgcolor="` + bgColor + `"
  label=<` + label + `>
  labelloc=t
  labeljust=l
  rankdir=LR
  newrank=true
  compound=true
  constraint=false
  ordering=out
  nodesep=0.05
  ranksep="2.0"
  node [fontsize=9 height=0.2 width=1.5 style=filled fontcolor="` + bgColor + `"]
  edge [arrowsize=0.5 color="` + fgColor + `"]
  subgraph hosts {
    cluster=true label="External Connections" rank=same ` + cluster +
		hosts + `
  }
  subgraph processes {
	cluster=true label=Processes ` + cluster +
		strings.Join(processes, "") + `
  }
  subgraph files {
	cluster=true label="Open Files" ` + cluster +
		datas + `
  }` +
		clusterEdges +
		hostEdges +
		strings.Join(processEdges, "") +
		dataEdges + `
}`)
}

// dot calls the Graphviz dot command to render the process NodeGraph as SVG.
func dot(graphviz string) []byte {
	cmd := exec.Command("dot", "-v", "-Tsvgz")
	cmd.Stdin = bytes.NewBufferString(graphviz)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		core.LogError(fmt.Errorf("dot command failed %w\n%s", err, stderr.Bytes()))
		sc := bufio.NewScanner(strings.NewReader(graphviz))
		for i := 1; sc.Scan(); i++ {
			fmt.Fprintf(os.Stderr, "%4.d %s\n", i, sc.Text())
		}
		return nil
	}

	return stdout.Bytes()
}

// parseQuery extracts the query from the HTTP request.
func parseQuery(r *http.Request) (query, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return query{}, err
	}
	var pid int
	if v, ok := values["pid"]; ok && len(v) > 0 {
		pid, _ = strconv.Atoi(v[0])
	}
	return query{
		pid: Pid(pid),
	}, nil
}

// family identifies all of the processes related to a process.
func family(pt process.Table, pid Pid) process.Table {
	ft := process.Table{pid: pt[pid]}
	for pid := pt[pid].Ppid; pid > 1; pid = pt[pid].Ppid { // ancestors
		ft[pid] = pt[pid]
	}

	pids := process.FlatTree(process.FindTree(process.BuildTree(pt), pid)) // descendants
	for _, pid := range pids {
		ft[pid] = pt[pid]
	}
	return ft
}

// longname formats the full Executable name and pid.
func longname(pt process.Table, pid Pid) string {
	if p, ok := pt[pid]; ok {
		name := p.Executable
		if name == "" {
			name = p.Id.Name
		}
		return fmt.Sprintf("%s[%d]", name, pid)
	}
	return ""
}

// shortname formats process name and pid.
func shortname(pt process.Table, pid Pid) string {
	if p, ok := pt[pid]; ok {
		return fmt.Sprintf("%s[%d]", p.Id.Name, pid)
	}
	return ""
}

// hostname resolves the host name for an ip address.
func hostname(ip string) string {
	hnLock.Lock()
	defer hnLock.Unlock()

	if host, ok := hnMap[ip]; ok {
		return host
	}

	hnMap[ip] = ip
	go func() { // initiate hostname lookup
		if hosts, err := net.LookupAddr(ip); err == nil {
			hnLock.Lock()
			hnMap[ip] = hosts[0]
			hnLock.Unlock()
		}
	}()

	return ip
}
