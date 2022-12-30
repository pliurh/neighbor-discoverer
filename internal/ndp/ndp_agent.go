package ndp

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/mdlayher/ndp"
	log "k8s.io/klog/v2"

	neighbordiscovererv1 "github.com/pliurh/neighbor-discoverer/api/v1"
)

// dropReason is the reason why a layer2 protocol packet was not
// responded to.
type dropReason int

// Various reasons why a packet was dropped.
const (
	dropReasonNone dropReason = iota
	dropReasonClosed
	dropReasonError
	dropReasonARPReply
	dropReasonMessageType
	dropReasonNoSourceLL
	dropReasonEthernetDestination
	dropReasonAnnounceIP
	dropReasonNotMatchInterface
)

type NdpAgent struct {
	logger        log.Logger
	linkLocalAddr netip.Addr
	hardwareAddr  net.HardwareAddr
	conn          *ndp.Conn
	closed        chan struct{}
	Mutex         *sync.Mutex
	Interface     *neighbordiscovererv1.InterfaceStatus
}

func NewNDPAgent(logger log.Logger, iface *neighbordiscovererv1.InterfaceStatus, mu *sync.Mutex) (*NdpAgent, error) {
	ifi, err := net.InterfaceByName(iface.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get interface with name %s: %s", iface.Name, err)
	}

	var lla *netip.Addr
	addrs, err := ifi.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get address for interface %s: %s", ifi.Name, err)
	}
	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			ip, _ := netip.ParseAddr(v.IP.String())
			if ip.IsLinkLocalUnicast() {
				lla = &ip
				break
			}
		default:
			continue
		}
	}
	if lla == nil {
		logger.V(0).Info("Linklocal address not found on", "interface", ifi.Name)
		return nil, nil
	}

	// Use link-local address as the source IPv6 address for NDP communications.
	conn, _, err := ndp.Listen(ifi, ndp.LinkLocal)
	if err != nil {
		return nil, fmt.Errorf("create NDP agent for interface %s: %s", ifi.Name, err)
	}

	n := &NdpAgent{
		logger:        logger,
		linkLocalAddr: *lla,
		hardwareAddr:  ifi.HardwareAddr,
		conn:          conn,
		Interface:     iface,
		Mutex:         mu,
	}
	go func() {
		for {
			n.logger.V(2).Info("Send unsolicited neighbor advertisement")
			n.sendUnsolicitedNA()
			time.Sleep(5 * time.Second)
		}
	}()
	go n.listen()
	return n, nil
}

func (n *NdpAgent) listen() {
	n.logger.V(0).Info("Listen for unsolicited neighbor advertisements")
	for n.processUnsolicitedNA() != dropReasonClosed {
	}
}

func (n *NdpAgent) processUnsolicitedNA() dropReason {
	msg, _, _, err := n.conn.ReadFrom()
	if err != nil {
		select {
		case <-n.closed:
			return dropReasonClosed
		default:
		}
		if err == io.EOF {
			return dropReasonClosed
		}
		return dropReasonError
	}

	// Only process Unsolicited Neighbor Advertisement
	na, ok := msg.(*ndp.NeighborAdvertisement)
	if !ok || na.Solicited {
		return dropReasonMessageType
	}

	// Retrieve sender's source link-layer address
	var naLLAddr net.HardwareAddr
	for _, o := range na.Options {
		// Ignore other options, including target link-layer address instead of source.
		lla, ok := o.(*ndp.LinkLayerAddress)
		if !ok {
			continue
		}
		if lla.Direction != ndp.Target {
			continue
		}

		naLLAddr = lla.Addr
		break
	}
	if naLLAddr == nil {
		return dropReasonNoSourceLL
	}

	n.logger.V(0).Info("Receive unsolicited neighbor advertisement from", "MAC", naLLAddr)
	new := neighbordiscovererv1.Neighbor{HardwareAddr: naLLAddr.String()}

	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	if _, ok := neighbordiscovererv1.NeighborContains(n.Interface.Neighbors, new); !ok {
		n.Interface.Neighbors = append(n.Interface.Neighbors, new)
	}

	return dropReasonNone
}

func (n *NdpAgent) sendUnsolicitedNA() error {
	err := n.advertise(netip.IPv6LinkLocalAllNodes(), n.linkLocalAddr, true)
	return err
}

func (n *NdpAgent) advertise(dst, target netip.Addr, gratuitous bool) error {
	m := &ndp.NeighborAdvertisement{
		Solicited:     !gratuitous,
		Override:      gratuitous,
		TargetAddress: target,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Target,
				Addr:      n.hardwareAddr,
			},
		},
	}
	return n.conn.WriteTo(m, nil, dst)
}
