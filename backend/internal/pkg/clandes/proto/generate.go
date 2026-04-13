// Package proto contains Cap'n Proto generated Go bindings for the clandes RPC protocol.
// Schemas are copied from clandes/crates/clandes-proto/schema/ (do not edit .capnp files directly here;
// sync from upstream then re-run go generate).
//
//go:generate sh -c "CAPNP_STD=$(go env GOPATH)/pkg/mod/capnproto.org/go/capnp/v3@v3.1.0-alpha.2/std && capnp compile -I$CAPNP_STD -ogo --src-prefix=. *.capnp"
package proto
