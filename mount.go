//revive:disable:package-comments
package connectserver

import (
	"net"

	"connectrpc.com/connect/v2/connecthttp"
)

// Mount registers a route on HTTPHost.Mux for every procedure registered on
// Server. It reads the registrations as they are when it is called, which is
// why it is its own step after they are made. Calling it again does nothing.
func (s *Host) Mount() {
	s.mounted.Do(func() {
		connecthttp.Mount(s.HTTPHost.Mux, s.Server, s.transport...)
	})
}

// Serve mounts the procedures and delegates serving to HTTPHost.
func (s *Host) Serve(lis net.Listener) error {
	s.Mount()

	return s.HTTPHost.Serve(lis)
}
