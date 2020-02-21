package virtualchannelsrpc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"gopkg.in/macaroon-bakery.v2/bakery"

	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

const (
	// subServerName is the name of the sub rpc server. We'll use this name
	// to register ourselves, and we also require that the main
	// SubServerConfigDispatcher instance recognize it as the name of our
	// RPC service.
	subServerName = "VirtualChannelsRPC"
)

var (
	// macaroonOps are the set of capabilities that our minted macaroon (if
	// it doesn't already exist) will have.
	macaroonOps = []bakery.Op{
		{
			Entity: "virtualchannels",
			Action: "write",
		},
		{
			Entity: "virtualchannels",
			Action: "read",
		},
	}

	// macPermissions maps RPC calls to the permissions they require.
	macPermissions = map[string][]bakery.Op{
		"/virtualchannelsrpc.VirtualChannels/CreateVirtualChannel": {{
			Entity: "virtualchannels",
			Action: "write",
		}},
		"/virtualchannelsrpc.VirtualChannels/SubscribeVirtualForward": {{
			Entity: "virtualchannels",
			Action: "read",
		}},
		"/virtualchannelsrpc.VirtualChannels/SettleVirtualForward": {{
			Entity: "virtualchannels",
			Action: "write",
		}},
		"/virtualchannelsrpc.VirtualChannels/DeleteVirtualChannel": {{
			Entity: "virtualchannels",
			Action: "write",
		}},
		"/virtualchannelsrpc.VirtualChannels/ListVirtualChannels": {{
			Entity: "virtualchannels",
			Action: "read",
		}},
	}

	// DefaultInvoicesMacFilename is the default name of the invoices
	// macaroon that we expect to find via a file handle within the main
	// configuration file in this package.
	DefaultVirtualChannelsMacFilename = "virtualchannels.macaroon"
)

type forward struct {
	chanID  lnwire.ChannelID
	htlc    []byte
	timeout uint32
}

// Server is a sub-server of the main RPC server: the invoices RPC. This sub
// RPC server allows external callers to access the status of the invoices
// currently active within lnd, as well as configuring it at runtime.
type Server struct {
	quit          chan struct{}
	cfg           *Config
	subscriptions []chan forward
}

// A compile time check to ensure that Server fully implements the
// InvoicesServer gRPC service.
var _ VirtualChannelsServer = (*Server)(nil)

// New returns a new instance of the invoicesrpc Invoices sub-server. We also
// return the set of permissions for the macaroons that we may create within
// this method. If the macaroons we need aren't found in the filepath, then
// we'll create them on start up. If we're unable to locate, or create the
// macaroons we need, then we'll return with an error.
func New(cfg *Config) (*Server, lnrpc.MacaroonPerms, error) {
	// If the path of the invoices macaroon wasn't specified, then we'll
	// assume that it's found at the default network directory.
	macFilePath := filepath.Join(
		cfg.NetworkDir, DefaultVirtualChannelsMacFilename,
	)

	// Now that we know the full path of the macaroon, we can
	// check to see if we need to create it or not.
	if !lnrpc.FileExists(macFilePath) && cfg.MacService != nil {
		log.Infof("Baking macaroons for invoices RPC Server at: %v",
			macFilePath)

		// At this point, we know that the invoices macaroon doesn't
		// yet, exist, so we need to create it with the help of the
		// main macaroon service.
		invoicesMac, err := cfg.MacService.Oven.NewMacaroon(
			context.Background(), bakery.LatestVersion, nil,
			macaroonOps...,
		)
		if err != nil {
			return nil, nil, err
		}
		invoicesMacBytes, err := invoicesMac.M().MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		err = ioutil.WriteFile(macFilePath, invoicesMacBytes, 0644)
		if err != nil {
			os.Remove(macFilePath)
			return nil, nil, err
		}
	}

	server := &Server{
		cfg:  cfg,
		quit: make(chan struct{}, 1),
	}

	return server, macPermissions, nil
}

// Start launches any helper goroutines required for the Server to function.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) Start() error {
	return nil
}

// Stop signals any active goroutines for a graceful closure.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) Stop() error {
	close(s.quit)

	return nil
}

// Name returns a unique string representation of the sub-server. This can be
// used to identify the sub-server and also de-duplicate them.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) Name() string {
	return subServerName
}

// RegisterWithRootServer will be called by the root gRPC server to direct a sub
// RPC server to register itself with the main gRPC root server. Until this is
// called, each sub-server won't be able to have requests routed towards it.
//
// NOTE: This is part of the lnrpc.SubServer interface.
func (s *Server) RegisterWithRootServer(grpcServer *grpc.Server) error {
	// We make sure that we register it with the main gRPC server to ensure
	// all our methods are routed properly.
	RegisterVirtualChannelsServer(grpcServer, s)

	log.Debugf("Virtual Channels RPC server successfully registered with root " +
		"gRPC server")

	return nil
}

func (s *Server) CreateVirtualChannel(ctx context.Context, request *CreateVirtualChannelRequest) (*CreateVirtualChannelResponse, error) {

	if len(request.NodePubKey) != 33 {
		return nil, fmt.Errorf("failed to create virtual channel: node pub key must be 33 bytes")
	}

	if len(request.ChannelId) != 32 {
		return nil, fmt.Errorf("failed to create virtual channel: channel id must be 32 bytes")
	}

	var pubKey [33]byte
	copy(pubKey[:], request.NodePubKey[0:33])

	var chanId [32]byte
	copy(chanId[:], request.ChannelId[0:32])

	// FIXME: When we have a virtual link registry, we should generate the short channel id there
	// we would want to be sure that it never reaches an actual segwit block height to avoid matching with
	// real existing channels
	link := htlcswitch.NewVirtualLink(
		pubKey,
		chanId,
		s.cfg.Switch.ForwardPackets,
		s.cfg.Switch.CircuitModifier(),
		s.acceptedHTLC)
	err := s.cfg.Switch.AddLink(link)
	if err != nil {
		return nil, fmt.Errorf("failed to create virtual channel: %w", err)
	}

	log.Errorf("Created new channel!")

	return &CreateVirtualChannelResponse{}, nil
}

func (s *Server) acceptedHTLC(chanID lnwire.ChannelID, htlc lnwire.Message, timeout uint32) {

	var buf bytes.Buffer
	if err := htlc.Encode(&buf, 0); err != nil {
		panic(err)
	}

	forward := forward{
		chanID:  chanID,
		htlc:    buf.Bytes(),
		timeout: timeout,
	}

	// TODO: This feels like it needs a lock
	for _, subscriber := range s.subscriptions {
		subscriber <- forward
	}

}

func (s *Server) SubscribeVirtualForward(request *SubscribeVirtualForwardRequest,
	stream VirtualChannels_SubscribeVirtualForwardServer) error {

	forwards := make(chan forward)
	// TODO: This feels like it needs a lock
	s.subscriptions = append(s.subscriptions, forwards)

	for {
		select {
		case forward := <- forwards:
			_ = forward
			stream.Send(&VirtualForward{
				ChannelId: forward.chanID[:],
				Htlc:      forward.htlc,
				Timeout:   int64(forward.timeout),
			})

		case <-s.quit:
			break
		}
	}
}

func (s *Server) SettleVirtualForward(ctx context.Context, request *SettleVirtualForwardRequest) (*SettleVirtualForwardResponse, error) {

	if len(request.ChannelId) != 32 {
		return nil, fmt.Errorf("failed to settle virtual forward: channel id must be 32 bytes")
	}

	if len(request.Preimage) != lntypes.PreimageSize {
		return nil, fmt.Errorf("failed to settle virtual forward: preimage size must be %v", lntypes.PreimageSize)
	}

	var chanId [32]byte
	copy(chanId[:], request.ChannelId[0:32])

	link, err := s.cfg.Switch.GetLink(chanId)
	if err != nil {
		return nil, fmt.Errorf("failed to settle virtual forward: didnt find virtual channel: %w", err)
	}

	virtualLink, ok := link.(htlcswitch.VirtualLink)
	if !ok {
		return nil, fmt.Errorf("failed to settle virtual forward: didnt find virtual channel")
	}

	var preimage lntypes.Preimage
	copy(preimage[:], request.Preimage[:lntypes.PreimageSize])

	virtualLink.SettleHtlc(preimage)

	return &SettleVirtualForwardResponse{}, nil
}

func (s *Server) DeleteVirtualChannel(context.Context, *DeleteVirtualChannelRequest) (*DeleteVirtualChannelResponse, error) {
	panic("implement me")
}

func (s *Server) ListVirtualChannels(context.Context, *ListVirtualChannelsRequest) (*ListVirtualChannelsResponse, error) {
	panic("implement me")
}

/*
// SubscribeSingleInvoice returns a uni-directional stream (server -> client)
// for notifying the client of state changes for a specified invoice.
func (s *Server) SubscribeSingleInvoice(req *SubscribeSingleInvoiceRequest,
	updateStream Invoices_SubscribeSingleInvoiceServer) error {

	hash, err := lntypes.MakeHash(req.RHash)
	if err != nil {
		return err
	}

	invoiceClient, err := s.cfg.InvoiceRegistry.SubscribeSingleInvoice(hash)
	if err != nil {
		return err
	}
	defer invoiceClient.Cancel()

	for {
		select {
		case newInvoice := <-invoiceClient.Updates:
			rpcInvoice, err := CreateRPCInvoice(
				newInvoice, s.cfg.ChainParams,
			)
			if err != nil {
				return err
			}

			if err := updateStream.Send(rpcInvoice); err != nil {
				return err
			}

		case <-s.quit:
			return nil
		}
	}
}
*/
