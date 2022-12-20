package dispatcher

import (
	"context"
	"reflect"
	"sync"

	"github.com/spf13/viper"

	"github.com/pandoprojects/pando/common"
	"github.com/pandoprojects/pando/p2p"
	p2ptypes "github.com/pandoprojects/pando/p2p/types"
	"github.com/pandoprojects/pando/p2pl"

	log "github.com/sirupsen/logrus"
)

var logger *log.Entry = log.WithFields(log.Fields{"prefix": "dispatcher"})

//
// Dispatcher dispatches messages to approporiate destinations
//
type Dispatcher struct {
	p2pnet  p2p.Network
	p2plnet p2pl.Network

	// Life cycle
	wg      *sync.WaitGroup
	quit    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	stopped bool
}

// NewLDispatcher returns the pointer to the Dispatcher singleton
func NewDispatcher(p2pnet p2p.Network, p2plnet p2pl.Network) *Dispatcher {
	return &Dispatcher{
		p2pnet:  p2pnet,
		p2plnet: p2plnet,
		wg:      &sync.WaitGroup{},
	}
}

// Start is called when the dispatcher starts
func (dp *Dispatcher) Start(ctx context.Context) error {
	c, cancel := context.WithCancel(ctx)
	dp.ctx = c
	dp.cancel = cancel
	var err error

	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		err = dp.p2pnet.Start(c)
		if err != nil {
			return err
		}
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		err = dp.p2plnet.Start(c)
	}
	return err
}

// Stop is called when the dispatcher stops
func (dp *Dispatcher) Stop() {
	dp.cancel()
}

// Wait suspends the caller goroutine
func (dp *Dispatcher) Wait() {
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		dp.p2pnet.Wait()
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		dp.p2plnet.Wait()
	}
	dp.wg.Wait()
}

// GetInventory sends out the InventoryRequest
func (dp *Dispatcher) GetInventory(peerIDs []string, invreq InventoryRequest) {
	if len(peerIDs) == 0 {
		dp.broadcastToNeighbors(invreq.ChannelID, invreq, true /* never ask an rametronenterprise for inventory */)
	} else {
		dp.send(peerIDs, invreq.ChannelID, invreq)
	}
}

// SendInventory sends out the InventoryResponse
func (dp *Dispatcher) SendInventory(peerIDs []string, invrsp InventoryResponse) {
	if len(peerIDs) == 0 {
		dp.broadcastToNeighbors(invrsp.ChannelID, invrsp, false /* should send to both blockchain and rametronenterprises */)
	} else {
		dp.send(peerIDs, invrsp.ChannelID, invrsp)
	}
}

// GetData sends out the DataRequest
func (dp *Dispatcher) GetData(peerIDs []string, datareq DataRequest) {
	if len(peerIDs) == 0 {
		dp.broadcastToNeighbors(datareq.ChannelID, datareq, true /* never ask an rametronenterprise for data */)
	} else {
		dp.send(peerIDs, datareq.ChannelID, datareq)
	}
}

// SendData sends out the DataResponse
func (dp *Dispatcher) SendData(peerIDs []string, datarsp DataResponse) {
	if len(peerIDs) == 0 {
		if datarsp.ChannelID == common.ChannelIDProposal {
			dp.broadcastToNeighbors(datarsp.ChannelID, datarsp, false /* should send to both blockchain and rametronenterprises */)
		} else if datarsp.ChannelID == common.ChannelIDGuardian {
			dp.broadcastToNeighbors(datarsp.ChannelID, datarsp, false /* no need to send guardian votes to rametronenterprises */)
		} else if datarsp.ChannelID == common.ChannelIDRametronenterpriseVote {
			dp.broadcastToNeighbors(datarsp.ChannelID, datarsp, false /* should send to both blockchain and rametronenterprises */)
		} else if datarsp.ChannelID == common.ChannelIDAggregatedRametronenterpriseVotes {
			dp.broadcastToAll(datarsp.ChannelID, datarsp, true /* no need to send the aggregated rametronenterprise votes back to rametronenterprises */)
		} else if datarsp.ChannelID == common.ChannelIDHeader {
			dp.broadcastToAll(datarsp.ChannelID, datarsp, false /* should send to both blockchain and rametronenterprises */)
		} else {
			dp.broadcastToAll(datarsp.ChannelID, datarsp, true /* backward compatibility, only broadcast to blockchain nodes */)
		}
	} else {
		dp.send(peerIDs, datarsp.ChannelID, datarsp)
	}
}

// ID returns the ID of the node
func (dp Dispatcher) ID() string {
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		return dp.p2pnet.ID()
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		return dp.p2plnet.ID()
	}
	return ""
}

// TODO: for 1.3.0 upgrade only, delete it after the upgrade completed
// ID returns the ID of the node
func (dp Dispatcher) LibP2PID() string {
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		return dp.p2plnet.ID()
	}
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		return dp.p2pnet.ID()
	}
	return ""
}

// Peers returns the IDs of all peers
func (dp *Dispatcher) Peers(skipRametronenterprise bool) []string {
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		return dp.p2pnet.Peers(skipRametronenterprise)
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		return dp.p2plnet.Peers(skipRametronenterprise)
	}
	return []string{}
}

// Peers returns the IDs of all peers
func (dp *Dispatcher) PeerURLs(skipRametronenterprise bool) []string {
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		return dp.p2pnet.PeerURLs(skipRametronenterprise)
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		return dp.p2plnet.PeerURLs(skipRametronenterprise)
	}
	return []string{}
}

// PeerExists indicates if the given peerID is a neighboring peer
func (dp *Dispatcher) PeerExists(peerID string) bool {
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		return dp.p2pnet.PeerExists(peerID)
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		return dp.p2plnet.PeerExists(peerID)
	}
	return false
}

// send delivers message directly to a list of peers.
func (dp *Dispatcher) send(peerIDs []string, channelID common.ChannelIDEnum, content interface{}) {
	messageOld := p2ptypes.Message{
		ChannelID: channelID,
		Content:   content,
	}
	message := p2ptypes.Message{
		ChannelID: channelID,
		Content:   content,
	}

	for _, peerID := range peerIDs {
		go func(peerID string) {
			if !reflect.ValueOf(dp.p2pnet).IsNil() {
				ok := dp.p2pnet.Send(peerID, messageOld)
				if !ok {
					logger.Debugf("Failed to send message to [%v]: %v, %v", peerID, channelID, content)
				}
			}
			if !reflect.ValueOf(dp.p2plnet).IsNil() {
				dp.p2plnet.Send(peerID, message)
			}
		}(peerID)
	}
}

// broadcastToAll publishes given message through gossip. Usually the message is only immediately delivered to
// a subset of neighbors.
func (dp *Dispatcher) broadcastToAll(channelID common.ChannelIDEnum, content interface{}, skipRametronenterprise bool) {
	messageOld := p2ptypes.Message{
		ChannelID: channelID,
		Content:   content,
	}
	message := p2ptypes.Message{
		ChannelID: channelID,
		Content:   content,
	}
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		dp.p2pnet.Broadcast(messageOld, skipRametronenterprise)
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		dp.p2plnet.Broadcast(message, skipRametronenterprise)
	}
}

// broadcastToNeighbors delivers given message to all neighbors.
func (dp *Dispatcher) broadcastToNeighbors(channelID common.ChannelIDEnum, content interface{}, skipRametronenterprise bool) {
	messageOld := p2ptypes.Message{
		ChannelID: channelID,
		Content:   content,
	}
	message := p2ptypes.Message{
		ChannelID: channelID,
		Content:   content,
	}
	maxNumPeersToBroadcast := viper.GetInt(common.CfgP2PMaxNumPeersToBroadcast)
	if !reflect.ValueOf(dp.p2pnet).IsNil() {
		//dp.p2pnet.Broadcast(messageOld)
		dp.p2pnet.BroadcastToNeighbors(messageOld, maxNumPeersToBroadcast, skipRametronenterprise)
	}
	if !reflect.ValueOf(dp.p2plnet).IsNil() {
		dp.p2plnet.BroadcastToNeighbors(message, maxNumPeersToBroadcast, skipRametronenterprise)
	}
}
