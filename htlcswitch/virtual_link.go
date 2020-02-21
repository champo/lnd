package htlcswitch

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/wire"

	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/lnpeer"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

type virtualLink struct {
	mailBox         MailBox
	id              lnwire.ChannelID
	peer            *virtualPeer
	packet          *htlcPacket
	forwardPackets  func(chan struct{}, ...*htlcPacket) chan error
	quit            chan struct{}
	circuitModifier CircuitModifier
	htlcAccepted    func(lnwire.ChannelID, lnwire.Message, uint32)
}

type VirtualLink interface {
	SettleHtlc(preimage lntypes.Preimage) error
}

func NewVirtualLink(pubKey [33]byte, chanId [32]byte,
	fwdPackets func(chan struct{}, ...*htlcPacket) chan error,
	circuitModifier CircuitModifier,
	htlcAccepted func(lnwire.ChannelID, lnwire.Message, uint32)) ChannelLink {

	identityKey, err := btcec.ParsePubKey(pubKey[:], btcec.S256())
	if err != nil {
		panic(fmt.Sprintf("Failed to parse pubkey for peer: %v", err))
	}

	// FIXME: I need an HTLCNotifier instance so I can tell it all about the HTLCs here
	return &virtualLink{
		id: chanId,
		peer: &virtualPeer{
			pubKey:      pubKey,
			identityKey: identityKey,
		},
		forwardPackets: fwdPackets,
		circuitModifier: circuitModifier,
		htlcAccepted: htlcAccepted,
	}
}

func (v *virtualLink) SettleHtlc(preimage lntypes.Preimage) error {
	log.Errorf("Settling %v", preimage.String())

	log.Errorf("Out %v In %v", v.packet.outgoingChanID, v.packet.incomingChanID)

	settlePacket := &htlcPacket{
		outgoingChanID: v.ShortChanID(),
		outgoingHTLCID: v.packet.outgoingHTLCID,
		incomingHTLCID: v.packet.incomingHTLCID,
		incomingChanID: v.packet.incomingChanID,
		htlc: &lnwire.UpdateFulfillHTLC{
			PaymentPreimage: preimage,
		},
	}

	errChan := v.forwardPackets(v.quit, settlePacket)
	go func() {
		for {
			err, ok := <-errChan
			if !ok {
				// Err chan has been drained or switch is shutting
				// down.  Either way, return.
				return
			}

			if err == nil {
				continue
			}

			log.Errorf("unhandled error while forwarding htlc packet over htlcswitch: %v", err)
		}
	}()

	return nil
}

func (v *virtualLink) HandleSwitchPacket(packet *htlcPacket) error {
	log.Errorf("HandleSwitchPacket %v", packet)
	v.packet = packet
	v.circuitModifier.OpenCircuits(packet.keystone())

	htlc := packet.htlc.(*lnwire.UpdateAddHTLC)
	log.Errorf("Payment hash = %v\n", hex.EncodeToString(htlc.PaymentHash[:]))

	// This id actually needs to increment! Otherwise we use already existing circuits
	// That, or we have to, you know, close circuits :D
	packet.outgoingHTLCID = 0
	packet.outgoingChanID = v.ShortChanID()

	go v.htlcAccepted(v.id, packet.htlc, packet.outgoingTimeout)

	// TODO: We need to be able to cancel forwards at some point
	return nil
}

func (v *virtualLink) HandleChannelUpdate(msg lnwire.Message) {
	log.Errorf("HandleChannelUpdate %v", msg)
}

func (v *virtualLink) ChanID() lnwire.ChannelID {
	log.Errorf("Got asked for ChanID")
	return v.id
}

func (v *virtualLink) ShortChanID() lnwire.ShortChannelID {
	log.Errorf("Got asked for ShortChanID")
	short := lnwire.ShortChannelID{
		BlockHeight: 1,
		TxIndex:     1,
		TxPosition:  0,
	}
	log.Errorf("short chan id = %v", short.ToUint64())

	return short
}

func (v *virtualLink) UpdateShortChanID() (lnwire.ShortChannelID, error) {
	// Noop
	return v.ShortChanID(), nil
}

func (v *virtualLink) UpdateForwardingPolicy(ForwardingPolicy) {
	// Noop
}

func (v *virtualLink) CheckHtlcForward(payHash [32]byte, incomingAmt lnwire.MilliSatoshi,
	amtToForward lnwire.MilliSatoshi, incomingTimeout, outgoingTimeout uint32, heightNow uint32) LinkError {
	log.Errorf("CheckHtlcForward %v", hex.EncodeToString(payHash[:]))
	return nil
}

func (v *virtualLink) CheckHtlcTransit(payHash [32]byte, amt lnwire.MilliSatoshi,
	timeout uint32, heightNow uint32) LinkError {
	log.Errorf("CheckHtlcTransit %v", hex.EncodeToString(payHash[:]))
	return nil
}

func (v *virtualLink) Bandwidth() lnwire.MilliSatoshi {
	// There's no good value here, so we just settle for a fixed value
	// This is 0.1BTC
	return 100000000 * 1000
}

func (v *virtualLink) Stats() (uint64, lnwire.MilliSatoshi, lnwire.MilliSatoshi) {
	// FIXME: record and return stats
	return 0, 0, 0
}

func (v *virtualLink) Peer() lnpeer.Peer {
	return v.peer
}

func (v *virtualLink) EligibleToForward() bool {
	return true
}

func (v *virtualLink) AttachMailBox(mailBox MailBox) {
	// FIXME: The mailbox is a mandatory thing, other subsystems might write directly to it
	v.mailBox = mailBox
}

func (v *virtualLink) Start() error {
	// TODO: Needs impl
	return nil
}

func (v *virtualLink) Stop() {
	// TODO: Needs impl
	close(v.quit)
}

type virtualPeer struct {
	pubKey      [33]byte
	identityKey *btcec.PublicKey
}

func (vp *virtualPeer) SendMessage(sync bool, msgs ...lnwire.Message) error {
	// This would seem to be used by subsystems other than the htlcswitch, so it should be safe to not implement
	panic("Cant send messages to virtual peers")
}

func (vp *virtualPeer) SendMessageLazy(sync bool, msgs ...lnwire.Message) error {
	// This would seem to be used by subsystems other than the htlcswitch, so it should be safe to not implement
	panic("Cant send messages to virtual peers")
}

func (vp *virtualPeer) AddNewChannel(channel *channeldb.OpenChannel, cancel <-chan struct{}) error {
	return errors.New("can't add a channel to a virtual link")
}

func (vp *virtualPeer) WipeChannel(*wire.OutPoint) error {
	return errors.New("can't wipe a channel from a virtual link")
}

func (vp *virtualPeer) PubKey() [33]byte {
	return vp.pubKey
}

func (vp *virtualPeer) IdentityKey() *btcec.PublicKey {
	return vp.identityKey
}

func (vp *virtualPeer) Address() net.Addr {
	panic("can't ask the Address from a virtualPeer")
}

func (vp *virtualPeer) QuitSignal() <-chan struct{} {
	panic("can't ask the QuitSignal from a virtualPeer")
}

func (vp *virtualPeer) LocalFeatures() *lnwire.FeatureVector {
	return lnwire.NewFeatureVector(nil, nil)
}

func (vp *virtualPeer) RemoteFeatures() *lnwire.FeatureVector {
	// TODO: When we implement MPP we have to declare that here
	return lnwire.NewFeatureVector(nil, nil)
}
