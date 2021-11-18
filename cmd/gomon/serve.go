// Copyright © 2021 The Gomon Project.

package main

import (
	"bytes"
	"net/http"
	"net/url"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zosmac/gomon/core"
	"github.com/zosmac/gomon/process"
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
		core.Flags.Sample = si // sync sample interval default with Prometheus'
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
			w.Write(process.NodeGraph(r))
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

					if err := websocket.Message.Send(ws, process.NodeGraph(ws.Request())); err != nil {
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

// serve sets up the web REST endpoints.
func serve() {
	// define http request handlers
	prometheusHandler()
	gomonHandler()
	wsHandler()

	go func() {
		port := strconv.Itoa(core.Flags.Port)
		core.LogError(http.ListenAndServe("localhost:"+port, nil))
		// to enable https/wss for these handlers, follow these steps:
		// 1. cmd/generate_cert/generate_cert -host localhost
		// 2. cp cmd/generate_cert/cert.pem cmd_generate_cert/key.pem ~/Developer/testdir
		// 3. add cert.pem to keychain
		// 4. in Safari, visit https://localhost:1234
		// 5. authorize untrusted self-signed certificate
		// core.LogError(http.ListenAndServeTLS("localhost:"+port, "cert.pem", "key.pem", nil))
	}()
}
