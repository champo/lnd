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

// FailureDetailHTLCExceedsMax is returned when a htlc exceeds our policy's
// maximum htlc amount.
type FailureDetailHTLCExceedsMax struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailHTLCExceedsMax) FailureString() string {
	return "htlc exceeds maximum policy value"
}

// FailureDetailInsufficientBalance is returned when we cannot route a
// htlc due to insufficient outgoing capacity.
type FailureDetailInsufficientBalance struct {
	// Available is the current balance available in the channel.
	Available lnwire.MilliSatoshi

	// Amount is the amount that we could not forward.
	Amount lnwire.MilliSatoshi
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part FailureDetailInsufficientBalance the FailureDetail interface.
func (fd *FailureDetailInsufficientBalance) FailureString() string {
	return fmt.Sprintf("insufficient balance: %v for payment: %v",
		fd.Available, fd.Amount)
}

// FailureDetailHTLCAddFailed is returned when we had an internal error adding
// a htlc.
type FailureDetailHTLCAddFailed struct {
	// Err is the error that occurred when trying to add the htlc.
	Err error
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailHTLCAddFailed) FailureString() string {
	return fmt.Sprintf("unable to handle downstream add "+
		"HTLC: %v", fd.Err)
}
