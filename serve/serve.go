// Copyright © 2021-2023 The Gomon Project.

package serve

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

	"github.com/zosmac/gocore"
	"golang.org/x/net/websocket"

	// enable web server to handle /debug/pprof queries
	_ "net/http/pprof"
)

var (
	// scheme is http/s based on whether certicate and key are defined in the user's .ssh directory.
	scheme = "http" // default

	httpHeader = func() http.Header {
		header := http.Header{
			"Access-Control-Allow-Origin": []string{"http://localhost"},
			"Content-Type":                []string{"image/svg+xml"},
		}
		if OUTPUT_FORMAT == "svgz" {
			header.Add("Content-Encoding", "gzip")
		}
		return header
	}
)

// gomonHandler retrieves the process NodeGraph.
func gomonHandler() error {
	http.HandleFunc(
		"/gomon/",
		func(w http.ResponseWriter, r *http.Request) {
			measures.HTTPRequests++
			for key, values := range httpHeader() {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.Write(Nodegraph(r))
		},
	)
	measures.Endpoints = append(measures.Endpoints, "gomon")
	return nil
}

// wsHandler opens a web socket for delivering periodically an updated process NodeGraph.
func wsHandler() error {
	wsscheme := "ws"
	if scheme == "https" {
		wsscheme = "wss"
	}
	http.Handle(
		"/ws",
		websocket.Server{
			Config: websocket.Config{
				Location: &url.URL{
					Scheme: wsscheme,
					Host:   "localhost:1234",
					Path:   "/ws",
				},
				Origin: &url.URL{
					Scheme: scheme,
					Host:   "localhost",
				},
				Version: websocket.ProtocolVersionHybi,
				Header:  httpHeader(),
			},
			Handler: func(ws *websocket.Conn) {
				// TODO: want to make this interactive, but not too demanding
				// should it send a new graph every minute? or require user to
				// press a button? how then to govern user requests? timer?

				buf := make([]byte, websocket.DefaultMaxPayloadBytes)
				for {
					if err := websocket.Message.Receive(ws, &buf); err != nil {
						gocore.Error("websocket Receive", err).Warn()
						ws.Close()
						return
					} else if bytes.HasPrefix(buf, []byte("suspend")) {
						continue
					}

					if err := websocket.Message.Send(ws, Nodegraph(ws.Request())); err != nil {
						gocore.Error("websocket Send", err).Warn()
						ws.Close()
						return
					}
				}
			},
			Handshake: func(c *websocket.Config, r *http.Request) error {
				return nil
			},
		},
	)
	measures.Endpoints = append(measures.Endpoints, "ws")
	return nil
}

// assetHandler serves up files from the gomon assets directory
func assetHandler() error {
	_, n, _, _ := runtime.Caller(2)
	mod := gocore.Module(filepath.Dir(n))
	if _, err := os.Stat(filepath.Join(mod.Dir, "assets")); err != nil {
		return gocore.Error("http assets unresolved", err)
	}

	http.Handle("/assets/",
		http.FileServer(http.Dir(mod.Dir)),
	)
	measures.Endpoints = append(measures.Endpoints, "assets")
	return nil
}

// serve sets up gomon's endpoints and starts the server.
func Serve(ctx context.Context) {
	// define http request handlers
	if err := prometheusHandler(); err != nil {
		gocore.Error("prometheusHandler", err).Warn()
	}
	if err := gomonHandler(); err != nil {
		gocore.Error("gomonHandler", err).Warn()
	}
	if err := wsHandler(); err != nil {
		gocore.Error("wsHandler", err).Warn()
	}
	if err := assetHandler(); err != nil {
		gocore.Error("assetHandler", err).Warn()
	}

	server := &http.Server{
		Addr: "localhost:" + strconv.Itoa(flags.port),
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background()) // let server perform cleanup with timeout
	}()

	go func() {
		// to enable https/wss for these handlers, follow these steps:
		// 1. cd /usr/local/go/src/crypto/tls
		// 2. go build -o ~/go/bin generate_cert.go
		// 3. cd ~/.ssh
		// 4. generate_cert -host localhost
		// 5. openssl x509 -noout -text -in cert.pem
		// 6. add cert.pem to keychain
		// 7. in Safari, visit https://localhost:1234/gomon
		// 8. authorize untrusted self-signed certificate

		u, _ := user.Current()
		serve := func() error { return server.ListenAndServe() }
		certfile := filepath.Join(u.HomeDir, ".ssh", "cert.pem")
		keyfile := filepath.Join(u.HomeDir, ".ssh", "key.pem")
		if _, err := os.Stat(filepath.Join(u.HomeDir, ".ssh")); err == nil {
			if _, err := os.Stat(certfile); err == nil {
				if _, err := os.Stat(keyfile); err == nil {
					scheme = "https"
					serve = func() error { return server.ListenAndServeTLS(certfile, keyfile) }
				}
			}
		}
		gocore.Error("gomon server", nil, map[string]string{
			"listen": scheme + "://" + server.Addr,
		}).Info()
		gocore.Error("gomon server", serve()).Err()
	}()
	measures.Address = scheme + "://" + server.Addr
}
