package htlcswitch

import (
	"sync"
	"time"

	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/subscribe"
)

// Htlcnotifier notifies clients of HTLC forwards, failures and settles for
// HTLCs that the switch handles.  It takes subscriptions for its events and
// notifies them when HTLC events occur. These are served on a best-effort
// basis; events are not persisted, delivery is not guaranteed (in the event
// of a crash in the switch, forward events may be lost) and events will be
// replayed upon restart. Events consumed from this package should be
// de-duplicated, and not relied upon for critical operations.
type HTLCNotifier struct {
	started sync.Once
	stopped sync.Once

	ntfnServer *subscribe.Server
}

// NewHTLCNotifier creates a new HTLCNotifier which gets HTLC forwarded, failed
// and settled events from links our node has established with peers and sends
// notifications to subscribing clients.
func NewHTLCNotifier() *HTLCNotifier {
	return &HTLCNotifier{
		ntfnServer: subscribe.NewServer(),
	}
}

// Start starts the HTLCNotifier and all goroutines it needs to consume events
// and provide subscriptions to clients.
func (h *HTLCNotifier) Start() error {
	var err error
	h.started.Do(func() {
		log.Trace("HTLCNotifier starting")
		err = h.ntfnServer.Start()
	})
	return err
}

// Stop signals the notifier for a graceful shutdown.
func (h *HTLCNotifier) Stop() {
	h.stopped.Do(func() {
		if err := h.ntfnServer.Stop(); err != nil {
			log.Warnf("error stopping htlc notifier: %v", err)
		}
	})
}

// SubscribeHTLCEvents returns a subscribe.Client that will receive updates
// any time the Server is made aware of a new event.
func (h *HTLCNotifier) SubscribeHTLCEvents() (*subscribe.Client, error) {
	return h.ntfnServer.Subscribe()
}

// HTLCKey uniquely identifies a htlc event. It can be used to associate forward
// events with their subsequent settle/fail event. Note that when matching a
// forward event with its subsequent settle/fail, the key will be reversed,
// because HTLCs are forwarded in one direction, then settled or failed back in
// the reverse direction.
type HTLCKey struct {
	// IncomingChannel is the channel that the incoming HTLC arrived at our node
	// on.
	IncomingChannel lnwire.ShortChannelID

	// OutgoingChannel is the channel that the outgoing HTLC left our node on.
	OutgoingChannel lnwire.ShortChannelID

	// HTLCInIndex is the index of the incoming HTLC in the incoming channel.
	HTLCInIndex uint64

	// HTLCOutIndex is the index of the outgoing HTLC in the outgoing channel.
	HTLCOutIndex uint64
}

// HTLCInfo provides the details of a HTLC that our node has processed. For
// forwards, incoming and outgoing values are set, whereas sends and receives
// will only have outgoing or incoming details set.
type HTLCInfo struct {
	// IncomingTimelock is the time lock of the HTLC on our incoming channel.
	IncomingTimeLock uint32

	// OutgoingTimelock is the time lock the HTLC on our outgoing channel.
	OutgoingTimeLock uint32

	// IncomingAmt is the amount of the HTLC on our incoming channel.
	IncomingAmt lnwire.MilliSatoshi

	// OutgoingAmt is the amount of the HTLC on our outgoing channel.
	OutgoingAmt lnwire.MilliSatoshi

	// PaymentHash is the payment hash of the HTLC. Note that this value is not
	// guaranteed to be unique.
	PaymentHash lntypes.Hash

	// HTLCEventType indicates whether the event is part of a send from our node
	// receive to our node of forward through our node.
	HTLCEventType
}

// HTLCEventType represents the type of event that a settle or failure was a
// part of.
type HTLCEventType int

const (
	// HTLCEventTypeSend is set when a HTLC is settled or failed as part of a
	// send from our node.
	HTLCEventTypeSend HTLCEventType = iota

	// HTLCEventTypeReceive is set when a HTLC is settled or failed as a part of
	// a receive to our node.
	HTLCEventTypeReceive

	// HTLCEventTypeForward is set when a HTLC that was forwarded through our
	// node is settled or failed.
	HTLCEventTypeForward
)

// ForwardingEvent represents a HTLC that was forwarded through our node. Sends
// which originate from our node will report forward events with zero incoming
// channel IDs in their HTLCKey.
type ForwardingEvent struct {
	// HTLCKey contains the information which uniquely identifies this forward
	// event.
	HTLCKey

	// HTLCInfo contains further information about the HTLC.
	HTLCInfo

	// Timestamp is the time when this HTLC was forwarded.
	Timestamp time.Time
}

// LinkFailEvent describes a HTLC that failed on our incoming our outgoing link.
// The incomingFailed bool is true for failures on incoming links, and false
// for failures on outgoing links. The failure reason is provided by a lnwire
// failure message which is enriched with a failure detail in the cases where
// we do not disclose full failure reasons to the network.
type LinkFailEvent struct {
	// HTLCKey contains the information which uniquely identifies the failed
	// HTLC.
	HTLCKey

	// HTLCInfo contains further information about the HTLC.
	HTLCInfo

	// FailReason is the reason that we failed the HTLC.
	FailReason lnwire.FailureMessage

	// FailDetail provides further information about the failure.
	FailDetail FailureDetail

	// IncomingFailed is true if the HTLC was failed on an incoming link.
	// If it failed on the outgoing link, it is false.
	IncomingFailed bool

	// Timestamp is the time when the link failure occurred.
	Timestamp time.Time
}

// ForwardingFailEvent represents a HTLC failure which occurred down the line
// after we forwarded the HTLC through our node. An error is not included in
// this event because errors returned down the route are encrypted.
type ForwardingFailEvent struct {
	// HTLCKey contains the information which uniquely identifies the original
	// HTLC forwarded event.
	HTLCKey

	// HTLCInfo contains further information about the HTLC.
	HTLCInfo

	// OnChain is true if the HTLC was failed on chain.
	OnChain bool

	// Timestamp is the time when the forwarding failure was received.
	Timestamp time.Time
}

// SettleEvent represents a HTLC that was forwarded by our node and successfully
// settled.
type SettleEvent struct {
	// HTLCKey contains the information which uniquely identifies the original
	// HTLC forwarded event.
	HTLCKey

	// HTLCInfo contains further information about the HTLC.
	HTLCInfo

	// OnChain is set to true if the HTLC was settled on chain.
	OnChain bool

	// Timestamp is the time when this HTLC was settled.
	Timestamp time.Time
}

// NotifyForwardingEvent notifies the HTLCNotifier than a HTLC has been
// forwarded.
//
// Note this is part of the Notifier interface.
func (h *HTLCNotifier) NotifyForwardingEvent(pkt *htlcPacket, hash lntypes.Hash) {
	// If the packet has a non-zero incoming channel ID, it is a forward.
	// Sends that are forwarded from our node have zero incoming channels IDs
	// because we are the origin node. We do not forward receives to our node,
	// so we cannot have forwarding events for receives.
	eventType := HTLCEventTypeSend
	if pkt.incomingChanID.ToUint64() == 0 {
		eventType = HTLCEventTypeForward
	}

	event := ForwardingEvent{
		HTLCKey:   newHTLCKey(pkt),
		HTLCInfo:  newHTLCInfo(pkt, hash, eventType),
		Timestamp: time.Now(),
	}

	log.Tracef("Notifying forward event type: %v from %v(%v) to %v(%v)",
		event.HTLCEventType, event.IncomingChannel, event.HTLCInIndex,
		event.OutgoingChannel, event.HTLCOutIndex)

	if err := h.ntfnServer.SendUpdate(event); err != nil {
		log.Warnf("Unable to send forwarding event: %v", err)
	}
}

// NotifyLinkFailEvent notifies the HTLCNotifier that we have failed a HTLC on
// one of our links.
//
// Note this is part of the Notifier interface.
func (h *HTLCNotifier) NotifyLinkFailEvent(pkt *htlcPacket, hash lntypes.Hash,
	eventType HTLCEventType, incomingFailed bool) {

	// Sanity check that the packet's link failure is provided.
	if pkt.linkFailure == nil {
		log.Error("NotifyLinkFailEvent link failure should be non-nil")
		return
	}

	event := LinkFailEvent{
		HTLCKey:        newHTLCKey(pkt),
		HTLCInfo:       newHTLCInfo(pkt, hash, eventType),
		FailReason:     pkt.linkFailure.GetWireMessage(),
		Timestamp:      time.Now(),
		IncomingFailed: true,
	}

	// If the link failure is a DetailError which enriches the wire failure
	// with additional information, add the failure detail to the event.
	failDetail, ok := pkt.linkFailure.(*DetailError)
	if ok {
		event.FailDetail = failDetail.FailureDetail
	}

	log.Tracef("Notifying link failure event type: %v from %v(%v) to "+
		"%v(%v), incoming link: %v, with reason: %v, details: %T",
		event.HTLCEventType, event.IncomingChannel, event.HTLCInIndex,
		event.OutgoingChannel, event.HTLCOutIndex, event.IncomingFailed,
		event.FailReason, event.FailDetail)

	if err := h.ntfnServer.SendUpdate(event); err != nil {
		log.Warnf("Unable to send link fail event: %v", err)
	}
}

// NotifyForwardingFailEvent notifies the HTLCNotifier that a HTLC we forwarded
// has failed down the line.
//
// Note this is part of the Notifier interface.
func (h *HTLCNotifier) NotifyForwardingFailEvent(pkt *htlcPacket) {
	// If the packet has a non-zero outgoing channel ID, it is a forward. Sends
	// from our node that fail have zero outgoing channels IDs because we are
	// the final node. We do not forward receives to our node, so we cannot have
	// forwarding failures for receives.
	eventType := HTLCEventTypeSend
	if pkt.outgoingChanID.ToUint64() == 0 {
		eventType = HTLCEventTypeForward
	}

	event := ForwardingFailEvent{
		HTLCKey: newHTLCKey(pkt),
		// HTLC hash in not available for forwarding failures, it can be obtained
		// by matching the failure with its corresponding forward event.
		HTLCInfo:  newHTLCInfo(pkt, lntypes.Hash{}, eventType),
		Timestamp: time.Now(),
	}

	log.Tracef("Notifying forwarding failure event type: %v from %v(%v) "+
		"to %v(%v)", event.HTLCEventType, event.IncomingChannel,
		event.HTLCInIndex, event.OutgoingChannel, event.HTLCOutIndex)

	if err := h.ntfnServer.SendUpdate(event); err != nil {
		log.Warnf("Unable to send forwarding fail event: %v", err)
	}
}

// NotifySettleEvent notifies the HTLCNotifier that a HTLC that we committed to
// as part of a forward or a receive to our node has been settled.
//
// Note this is part of the Notifier interface.
func (h *HTLCNotifier) NotifySettleEvent(pkt *htlcPacket, hash lntypes.Hash,
	eventType HTLCEventType) {

	event := SettleEvent{
		HTLCKey:   newHTLCKey(pkt),
		HTLCInfo:  newHTLCInfo(pkt, hash, eventType),
		Timestamp: time.Now(),
	}

	log.Tracef("Notifying settle event type: %v from %v(%v) to %v(%v)",
		event.HTLCEventType, event.IncomingChannel, event.HTLCInIndex,
		event.OutgoingChannel, event.HTLCOutIndex)

	if err := h.ntfnServer.SendUpdate(event); err != nil {
		log.Warnf("Unable to send settle event: %v", err)
	}
}

// newHTLCKey returns a HTLCKey for the packet provided.
func newHTLCKey(pkt *htlcPacket) HTLCKey {
	return HTLCKey{
		IncomingChannel: pkt.incomingChanID,
		OutgoingChannel: pkt.outgoingChanID,
		HTLCInIndex:     pkt.incomingHTLCID,
		HTLCOutIndex:    pkt.outgoingHTLCID,
	}
}

// newHTLCInfo returns HTLCInfo for the packet provided.
func newHTLCInfo(pkt *htlcPacket, paymentHash lntypes.Hash,
	eventType HTLCEventType) HTLCInfo {

	return HTLCInfo{
		IncomingTimeLock: pkt.incomingTimeout,
		OutgoingTimeLock: pkt.outgoingTimeout,
		IncomingAmt:      pkt.incomingAmount,
		OutgoingAmt:      pkt.amount,
		PaymentHash:      paymentHash,
		HTLCEventType:    eventType,
	}
}
