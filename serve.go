// Copyright © 2021-2023 The Gomon Project.

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zosmac/gocore"
	"golang.org/x/net/websocket"

	// enable web server to handle /debug/pprof queries
	_ "net/http/pprof"
)

// prometheusHandler responds to Prometheus Collect requests.
func prometheusHandler() {
	// enable Prometheus collection (we don't use the default registry as it adds Go runtime metrics)
	registry := prometheus.NewRegistry()
	registry.MustRegister(&prometheusCollector{})
	http.Handle(
		"/metrics",
		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	)

	if si, err := scrapeInterval(); err == nil {
		flags.sample = si // sync sample interval default with Prometheus'
	}
}

// gomonHandler retrieves the process NodeGraph.
func gomonHandler() {
	http.HandleFunc(
		"/gomon/",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Access-Control-Allow-Origin", "http://localhost")
			w.Header().Add("Content-Type", "image/svg+xml")
			w.Header().Add("Content-Encoding", "gzip")
			w.Write(NodeGraph(r))
		},
	)
}

// wsHandler opens a web socket for delivering periodically an updated process NodeGraph.
func wsHandler() {
	http.Handle(
		"/ws",
		websocket.Server{
			Config: websocket.Config{
				Location: &url.URL{
					Scheme: "ws",
					Host:   "localhost:1234",
					Path:   "/ws",
				},
				Origin: &url.URL{
					Scheme: "http",
					Host:   "localhost",
				},
				Version: websocket.ProtocolVersionHybi,
				Header: http.Header{
					"Access-Control-Allow-Origin": []string{"http://localhost"},
					"Content-Type":                []string{"image/svg+xml"},
					"Content-Encoding":            []string{"gzip"},
				},
			},
			Handler: func(ws *websocket.Conn) {
				// TODO: want to make this interactive, but not too demanding
				// should it send a new graph every minute? or require user to
				// press a button? how then to govern user requests? timer?

				buf := make([]byte, websocket.DefaultMaxPayloadBytes)
				for {
					if err := websocket.Message.Receive(ws, &buf); err != nil {
						ws.Close()
						return
					}
					if bytes.HasPrefix(buf, []byte("suspend")) {
						continue
					}

					if err := websocket.Message.Send(ws, NodeGraph(ws.Request())); err != nil {
						return
					}
				}
			},
			Handshake: func(c *websocket.Config, r *http.Request) error {
				return nil
			},
		},
	)
}

// assetHandler serves up files from the gomon assets directory
func assetHandler() {
	_, n, _, _ := runtime.Caller(2)
	mod := gocore.Module(filepath.Dir(n))
	if _, err := os.Stat(filepath.Join(mod.Dir, "assets")); err != nil {
		gocore.LogWarn(gocore.Error("http assets unresolved", err))
		return
	}

	http.Handle("/assets/",
		http.FileServer(http.Dir(mod.Dir)),
	)
}

// serve sets up gomon's endpoints and starts the server.
func serve(ctx context.Context) {
	// define http request handlers
	prometheusHandler()
	gomonHandler()
	wsHandler()
	assetHandler()

	server := &http.Server{
		Addr: "localhost:" + strconv.Itoa(flags.port),
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background()) // let server perform cleanup with timeout
	}()

	go func() {
		// gocore.LogError(server.ListenAndServe())

		// to enable https/wss for these handlers, follow these steps:
		// 1. cd /usr/local/go/src/crypto/tls
		// 2. go build -o ~/go/bin generate_cert.go
		// 3. cd ~/.ssh
		// 4. generate_cert -host localhost
		// 5. add cert.pem to keychain
		// 6. in Safari, visit https://localhost:1234/gomon
		// 7. authorize untrusted self-signed certificate

		u, _ := user.Current()
		dir := filepath.Join(u.HomeDir, ".ssh")
		gocore.LogError(server.ListenAndServeTLS(
			filepath.Join(dir, "cert.pem"),
			filepath.Join(dir, "key.pem"),
		))
	}()
}
