package htlcswitch

import (
	"fmt"

	"github.com/lightningnetwork/lnd/lnwire"

	"github.com/lightningnetwork/lnd/lntypes"
)

// FailureDetail is an interface which is used to enrich failures with
// additional information.
type FailureDetail interface {
	// FailureString returns a string representation of a failure detail.
	FailureString() string
}

// FailureDetailOnionDecode indicates that we could not decode an onion
// error.
type FailureDetailOnionDecode struct {
	PaymentHash lntypes.Hash
	PaymentID   uint64
	Err         error
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailOnionDecode) FailureString() string {
	return fmt.Sprintf("unable to decode onion failure (hash=%v, "+
		"pid=%d): %v", fd.PaymentHash, fd.PaymentID, fd.Err)
}

// FailureDetailLinkNotEligible indicates that a routing attempt was made
// over a link that is not eligible for routing.
type FailureDetailLinkNotEligible struct {
	ChannelID lnwire.ShortChannelID
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailLinkNotEligible) FailureString() string {
	return fmt.Sprintf("link %v is not available to forward", fd.ChannelID)
}

// FailureDetailOnChainTimeout indicates that a payment had to be timed out
// on chain before it got past the first hop by us or the remote party.
type FailureDetailOnChainTimeout struct {
	PaymentHash lntypes.Hash
	PaymentID   uint64
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailOnChainTimeout) FailureString() string {
	return fmt.Sprintf("payment was resolved on-chain, then canceled back "+
		"(hash=%v, pid=%d)", fd.PaymentHash, fd.PaymentID)
}
