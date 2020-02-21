package virtualchannelsrpc

import (
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/macaroons"
)

// Config is the primary configuration struct for the RPC server. It
// contains all the items required for the rpc server to carry out its
// duties. The fields with struct tags are meant to be parsed as normal
// configuration options, while if able to be populated, the latter fields MUST
// also be specified.
type Config struct {
	// NetworkDir is the main network directory wherein the rpc
	// server will find the macaroon named DefaultVirtualChannelsMacFilename.
	NetworkDir string

	// MacService is the main macaroon service that we'll use to handle
	// authentication for the rpc server.
	MacService *macaroons.Service

	// Switch is used to register new virtual channels
	Switch *htlcswitch.Switch
}
