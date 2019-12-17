package htlcswitch

import (
	"bytes"
	"fmt"

	sphinx "github.com/lightningnetwork/lightning-onion"
	"github.com/lightningnetwork/lnd/htlcswitch/hop"
	"github.com/lightningnetwork/lnd/lnwire"
)

// LinkError is an interface which wraps lnwire failure messages for
// internal use within the switch. This interface is intended to create a
// barrier between the switch and wire.
type LinkError interface {
	GetWireMessage() lnwire.FailureMessage
	error
}

// WireError wraps a wire message for use within the switch.
type WireError struct {
	Msg lnwire.FailureMessage
}

// GetWireMessage unwraps a WireError to obtain a wire message.
//
// Note this is part of the LinkError interface.
func (s *WireError) GetWireMessage() lnwire.FailureMessage {
	return s.Msg
}

// Error returns an error string for a switch failure.
//
// Note this is part of the LinkError interface.
func (s *WireError) Error() string {
	return s.Msg.Error()
}

// NewWireError returns a switch error which wraps the wire message provided.
func NewWireError(msg lnwire.FailureMessage) LinkError {
	return &WireError{
		Msg: msg,
	}
}

// PaymentError is used to signal that one of our own payments has failed,
// either on our own outgoing link (source index=0) or further down the
// router (non-zero source index). It embeds the LinkError interface so that
// it may be used in the switch as a superset of LinkError.
type PaymentError struct {
	// FailureSourceIdx is the index of the node that sent the failure. With
	// this information, the dispatcher of a payment can modify their set of
	// candidate routes in response to the type of failure extracted. Index
	// zero is the self node.
	FailureSourceIdx int

	// ExtraMsg is an additional error message that callers can provide in
	// order to provide context specific error details.
	ExtraMsg string

	// LinkError is an internal error which wraps a wire message with
	// additional information.
	LinkError
}

// Error implements the built-in error interface. We use this method to allow
// the switch or any callers to insert additional context to the error message
// returned.
func (p *PaymentError) Error() string {
	if p.ExtraMsg == "" {
		return fmt.Sprintf(
			"%v@%v", p.GetWireMessage(), p.FailureSourceIdx,
		)
	}

	return fmt.Sprintf(
		"%v@%v: %v", p.GetWireMessage(), p.FailureSourceIdx, p.ExtraMsg,
	)
}

// NewPaymentError creates a new payment error which wraps a link error with
// additional metadata.
func NewPaymentError(failure LinkError, index int,
	extraMsg string) *PaymentError {

	return &PaymentError{
		FailureSourceIdx: index,
		LinkError:        failure,
		ExtraMsg:         extraMsg,
	}
}

// ErrorDecrypter is an interface that is used to decrypt the onion encrypted
// failure reason an extra out a well formed error.
type ErrorDecrypter interface {
	// DecryptError peels off each layer of onion encryption from the first
	// hop, to the source of the error. A fully populated
	// lnwire.FailureMessage is returned along with the source of the
	// error.
	DecryptError(lnwire.OpaqueReason) (*PaymentError, error)
}

// UnknownEncrypterType is an error message used to signal that an unexpected
// EncrypterType was encountered during decoding.
type UnknownEncrypterType hop.EncrypterType

// Error returns a formatted error indicating the invalid EncrypterType.
func (e UnknownEncrypterType) Error() string {
	return fmt.Sprintf("unknown error encrypter type: %d", e)
}

// OnionErrorDecrypter is the interface that provides onion level error
// decryption.
type OnionErrorDecrypter interface {
	// DecryptError attempts to decrypt the passed encrypted error response.
	// The onion failure is encrypted in backward manner, starting from the
	// node where error have occurred. As a result, in order to decrypt the
	// error we need get all shared secret and apply decryption in the
	// reverse order.
	DecryptError(encryptedData []byte) (*sphinx.DecryptedError, error)
}

// SphinxErrorDecrypter wraps the sphinx data SphinxErrorDecrypter and maps the
// returned errors to concrete lnwire.FailureMessage instances.
type SphinxErrorDecrypter struct {
	OnionErrorDecrypter
}

// DecryptError peels off each layer of onion encryption from the first hop, to
// the source of the error. A fully populated lnwire.FailureMessage is returned
// along with the source of the error.
//
// NOTE: Part of the ErrorDecrypter interface.
func (s *SphinxErrorDecrypter) DecryptError(reason lnwire.OpaqueReason) (
	*PaymentError, error) {

	failure, err := s.OnionErrorDecrypter.DecryptError(reason)
	if err != nil {
		return nil, err
	}

	// Decode the failure. If an error occurs, we leave the failure message
	// field nil.
	r := bytes.NewReader(failure.Message)
	failureMsg, err := lnwire.DecodeFailure(r, 0)
	if err != nil {
		return NewPaymentError(NewWireError(nil), failure.SenderIdx, ""), nil
	}

	return NewPaymentError(
		NewWireError(failureMsg), failure.SenderIdx, "",
	), nil
}

// A compile time check to ensure ErrorDecrypter implements the Deobfuscator
// interface.
var _ ErrorDecrypter = (*SphinxErrorDecrypter)(nil)
