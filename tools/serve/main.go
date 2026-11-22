// Command serve is a tiny web server for trying the browser version
// locally: `make serve` builds it into dist/ and serves it at
// http://localhost:8080. It isn't part of the game; the published game
// needs no server of its own.
//
// Usage:
//
//	go run ./tools/serve [-dir dist] [-addr localhost:8080]
package main

import (
	"flag"
	"log"
	"net/http"
)

// main serves -dir at -addr until stopped with Ctrl-C.
func main() {
	dir := flag.String("dir", "dist", "folder to serve")
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	flag.Parse()

	log.Printf("serving %s at http://%s (Ctrl-C to stop)", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, handler(*dir)))
}

// handler serves the files in dir. Every response says not to cache it,
// so a rebuilt game shows up on the next reload.
//
// Go note: http.FileServer picks each file's Content-Type from its
// extension, and Go knows .wasm is application/wasm. Browsers insist on
// that type for WebAssembly.instantiateStreaming. http.HandlerFunc turns
// an ordinary function into an http.Handler, so it can wrap the file
// server and add a header first.
func handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})
}
