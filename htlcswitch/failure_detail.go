package htlcswitch

import (
	"fmt"

	"github.com/lightningnetwork/lnd/invoices"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
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

// FailureDetailIncompleteForward is returned when we cancel an incomplete
// forward.
type FailureDetailIncompleteForward struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailIncompleteForward) FailureString() string {
	return "failing packet after detecting incomplete forward"
}

// FailureDetailCannotEncodeRoute is returned when we cannot encode the
// remainder of the route.
type FailureDetailCannotEncodeRoute struct {
	// Err is the error that occurred when trying to encode the route.
	Err error
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailCannotEncodeRoute) FailureString() string {
	return fmt.Sprintf("unable to encode the remaining route %v", fd.Err)
}

// FailureDetailInvoiceAlreadyCancelled is returned when an invoice has already
// been cancelled when an attempt to pay is is made.
type FailureDetailInvoiceAlreadyCancelled struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailInvoiceAlreadyCancelled) FailureString() string {
	return "invoice already cancelled"
}

// FailureDetailAmountTooLow is returned when an invoice is underpaid.
type FailureDetailAmountTooLow struct {
	// Amount is the amount that was paid.
	Amount lnwire.MilliSatoshi
}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailAmountTooLow) FailureString() string {
	return fmt.Sprintf("invoice underpaid, amount: %v", fd.Amount)
}

// FailureDetailExpiryTooSoon is returned when we do not accept an invoice
// payment because it expires too soon.
type FailureDetailExpiryTooSoon struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailExpiryTooSoon) FailureString() string {
	return "invoice expiry too soon"
}

// FailureDetailInvoiceNotOpen is returned when an attempt is made to pay a mpp
// invoice which is not open.
type FailureDetailInvoiceNotOpen struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailInvoiceNotOpen) FailureString() string {
	return "mpp invoice not open"
}

// FailureDetailAddressMismatch is returned when the payment address for a mpp
// invoice does not match.
type FailureDetailAddressMismatch struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailAddressMismatch) FailureString() string {
	return "mpp address mismatch"
}

// FailureDetailSetTotalMismatch is returned when the amount paid by a htlc
// does not match its set total.
type FailureDetailSetTotalMismatch struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailSetTotalMismatch) FailureString() string {
	return "mpp set total mismatch"
}

// FailureDetailSetTotalTooLow is returned when a mpp set total is too low for
// an invoice.
type FailureDetailSetTotalTooLow struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailSetTotalTooLow) FailureString() string {
	return "mpp set total too low"
}

// FailureDetailSetOverpaid is returned when a mpp set is overpaid.
type FailureDetailSetOverpaid struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailSetOverpaid) FailureString() string {
	return "mpp set overpaid"
}

// FailureDetailInvoiceNotFound is returned when an attempt is made to pay an
// invoice that is unknown to us.
type FailureDetailInvoiceNotFound struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailInvoiceNotFound) FailureString() string {
	return "invoice not found"
}

// FailureDetailHodlCancel is returned when a hodl invoice is cancelled.
type FailureDetailHodlCancel struct{}

// FailureString returns a string representation of a failure detail.
//
// Note it is part of the FailureDetail interface.
func (fd *FailureDetailHodlCancel) FailureString() string {
	return "hodl invoice cancelled"
}

// getFailureDetail returns a failure detail that is associated with a
// resolution result. It defaults to FailureReasonHodlCancel in the
// absence of a specific failure reason.
func getFailureDetail(result invoices.ResolutionResult,
	amount lnwire.MilliSatoshi) FailureDetail {

	switch result {
	case invoices.ResultInvoiceAlreadyCanceled:
		return &FailureDetailInvoiceAlreadyCancelled{}

	case invoices.ResultAmountTooLow:
		return &FailureDetailAmountTooLow{
			Amount: amount,
		}

	case invoices.ResultExpiryTooSoon:
		return &FailureDetailExpiryTooSoon{}

	case invoices.ResultInvoiceNotOpen:
		return &FailureDetailInvoiceNotOpen{}

	case invoices.ResultAddressMismatch:
		return &FailureDetailAddressMismatch{}

	case invoices.ResultHtlcSetTotalMismatch:
		return &FailureDetailSetTotalMismatch{}

	case invoices.ResultHtlcSetTotalTooLow:
		return &FailureDetailSetTotalTooLow{}

	case invoices.ResultHtlcSetOverpayment:
		return &FailureDetailSetOverpaid{}

	case invoices.ResultInvoiceNotFound:
		return &FailureDetailInvoiceNotFound{}

	default:
		return &FailureDetailHodlCancel{}
	}
}
