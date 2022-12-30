package syncer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	log "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// "github.com/containerd/containerd/pkg/timeout"
	neighbordiscovererv1 "github.com/pliurh/neighbor-discoverer/api/v1"
	"github.com/pliurh/neighbor-discoverer/internal/ndp"
)

type NodeNeighborStatusSyncer struct {
	client    client.Client
	nodeName  string
	namespace string
	interval  int
	timeout   int
	status    *neighbordiscovererv1.NodeNeighborNetworkStatus
	Mutex     sync.Mutex
	ndpAgents []*ndp.NdpAgent
}

func NewNodeNeighborStatusSyncer(c client.Client, node string, ns string, i, t int) *NodeNeighborStatusSyncer {

	return &NodeNeighborStatusSyncer{
		client:    c,
		nodeName:  node,
		namespace: ns,
		interval:  i,
		timeout:   t,
		status:    &neighbordiscovererv1.NodeNeighborNetworkStatus{},
	}
}

func (s *NodeNeighborStatusSyncer) Run(ctx context.Context) error {
	log.V(0).Infof("Start syncer")
	time.Sleep(3 * time.Second)
	if err := s.initLocalNodeNeighborNetwork(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			log.V(0).Info("Stop syncer")
			return nil

		case <-time.After(time.Duration(s.interval) * time.Second):
			log.V(0).Info("Period sync status")

			err := s.SyncNodeNeighborStatus(ctx)
			if err != nil {
				log.Errorf("Failed to sync status: %v", err)
				return err
			}
		}
	}
}

func (s *NodeNeighborStatusSyncer) initLocalNodeNeighborNetwork(ctx context.Context) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		neighborNetwork := &neighbordiscovererv1.NodeNeighborNetwork{}
		key := types.NamespacedName{Namespace: s.namespace, Name: s.nodeName}
		err = s.client.Get(ctx, key, neighborNetwork, &client.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.V(0).Info("Create local NodeNeighborNetwork CR")
				neighborNetwork.ObjectMeta = metav1.ObjectMeta{
					Name:      s.nodeName,
					Namespace: s.namespace,
				}
				if err := s.client.Create(ctx, neighborNetwork, &client.CreateOptions{}); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		log.V(0).Info("Initial NodeNeighborNetwork status with local network devices info")
		_, err = DiscoverNetworkDevices(s.status)
		if err != nil {
			return err
		}
		neighborNetwork.Status = *s.status
		if err := s.client.Status().Update(ctx, neighborNetwork, &client.UpdateOptions{}); err != nil {
			return err
		}

		// start a NDPAgent for each interface
		for i, iface := range s.status.Interfaces {
			logr := log.NewKlogr().WithName(iface.Name)

			agent, err := ndp.NewNDPAgent(logr, &s.status.Interfaces[i], &s.Mutex)
			if err != nil {
				return err
			}
			if agent != nil {
				s.ndpAgents = append(s.ndpAgents, agent)
			}
		}
		return nil
	})
	if err != nil {
		// may be conflict if max retries were hit
		return fmt.Errorf("unable to create node neighbor network CR: %v", err)
	}
	return nil
}

func (s *NodeNeighborStatusSyncer) SyncNodeNeighborStatus(ctx context.Context) error {
	// Sync Status
	return s.updateNodeNeighborStatusRetry(ctx)
}

func (s *NodeNeighborStatusSyncer) updateNodeNeighborStatusRetry(ctx context.Context) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		neighborNetworkList := &neighbordiscovererv1.NodeNeighborNetworkList{}
		var snn *neighbordiscovererv1.NodeNeighborNetwork
		err = s.client.List(ctx, neighborNetworkList, &client.ListOptions{Namespace: s.namespace})
		if err != nil {
			return err
		}

		s.Mutex.Lock()
		defer s.Mutex.Unlock()
		for _, agent := range s.ndpAgents {
			for i, _ := range agent.Interface.Neighbors {
				for _, nn := range neighborNetworkList.Items {
					if nn.Name == s.nodeName {
						continue
					}
					for _, iface := range nn.Status.Interfaces {
						if agent.Interface.Neighbors[i].HardwareAddr == iface.HardwareAddr {
							agent.Interface.Neighbors[i].InterfaceName = iface.Name
							agent.Interface.Neighbors[i].NodeName = nn.Name
							break
						}
					}
				}
			}
		}

		for _, nn := range neighborNetworkList.Items {
			if nn.Name == s.nodeName {
				snn = &nn
				break
			}
		}
		if equality.Semantic.DeepEqual(snn.Status, *s.status) {
			return nil
		}

		log.V(0).Info("Neighbor status changed, update the custom resource")
		snn.Status = *s.status
		err = s.client.Status().Update(ctx, snn, &client.UpdateOptions{})
		if err != nil {
			log.V(0).Infof("Fail to update the node status: %v", err)
		}
		return err
	})
	if err != nil {
		// may be conflict if max retries were hit
		return fmt.Errorf("unable to update node neighbor status: %v", err)
	}

	return nil
}
