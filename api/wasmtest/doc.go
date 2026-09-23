// Package wasmtest runs the api package's wasm-only renderers under
// GOOS=js GOARCH=wasm (`make test-wasm`). It is a separate package because
// ginkgo does not compile for js (its progress reporter needs
// syscall.SIGUSR1), which would stop every test in package api; the tests
// here therefore use plain testing and only the exported api.
package wasmtest
