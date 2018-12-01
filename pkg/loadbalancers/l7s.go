/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package loadbalancers

import (
	"fmt"
	"reflect"

	mcrt "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/gke.googleapis.com/v1alpha1"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/ingress-gce/pkg/events"
	"k8s.io/ingress-gce/pkg/storage"
	"k8s.io/ingress-gce/pkg/utils"
)

// L7s implements LoadBalancerPool.
type L7s struct {
	cloud            LoadBalancers
	snapshotter      storage.Snapshotter
	namer            *utils.Namer
	mcrt             mcrt.ManagedCertificateLister
	recorderProducer events.RecorderProducer
}

// Namer returns the namer associated with the L7s.
func (l *L7s) Namer() *utils.Namer {
	return l.namer
}

// NewLoadBalancerPool returns a new loadbalancer pool.
// - cloud: implements LoadBalancers. Used to sync L7 loadbalancer resources
//	 with the cloud.
func NewLoadBalancerPool(cloud LoadBalancers, namer *utils.Namer, mcrt mcrt.ManagedCertificateLister, recorderProducer events.RecorderProducer) LoadBalancerPool {
	return &L7s{cloud, storage.NewInMemoryPool(), namer, mcrt, recorderProducer}
}

// Get returns the loadbalancer by name.
func (l *L7s) Get(name string) (*L7, error) {
	name = l.namer.LoadBalancer(name)
	lb, exists := l.snapshotter.Get(name)
	if !exists {
		return nil, fmt.Errorf("loadbalancer %v not in pool", name)
	}
	return lb.(*L7), nil
}

// Sync a load balancer with the given runtime info from the controller.
func (l *L7s) Sync(ri *L7RuntimeInfo) error {
	name := l.namer.LoadBalancer(ri.Name)

	lb, _ := l.Get(name)
	if lb == nil {
		glog.V(3).Infof("Creating l7 %v", name)
		lb = &L7{
			runtimeInfo: ri,
			Name:        l.namer.LoadBalancer(ri.Name),
			cloud:       l.cloud,
			namer:       l.namer,
			mcrt:        l.mcrt,
			recorder:    l.recorderProducer.Recorder(ri.IngressList.Items[0].Namespace),
		}
	} else {
		if !reflect.DeepEqual(lb.runtimeInfo, ri) {
			glog.V(3).Infof("LB %v runtime info changed, old %+v new %+v", lb.Name, lb.runtimeInfo, ri)
			lb.runtimeInfo = ri
		}
	}
	// Add the lb to the pool, in case we create an UrlMap but run out
	// of quota in creating the ForwardingRule we still need to cleanup
	// the UrlMap during GC.
	defer l.snapshotter.Add(name, lb)

	// Why edge hop for the create?
	// The loadbalancer is a fictitious resource, it doesn't exist in gce. To
	// make it exist we need to create a collection of gce resources, done
	// through the edge hop.
	if err := lb.edgeHop(); err != nil {
		return err
	}

	return nil
}

// Delete deletes a load balancer by name.
func (l *L7s) Delete(name string) error {
	name = l.namer.LoadBalancer(name)
	lb, err := l.Get(name)
	if err != nil {
		return err
	}
	glog.V(3).Infof("Deleting lb %v", name)
	if err := lb.Cleanup(); err != nil {
		return err
	}
	l.snapshotter.Delete(name)
	return nil
}

// GC garbage collects loadbalancers not in the input list.
func (l *L7s) GC(names []string) error {
	glog.V(4).Infof("GC(%v)", names)

	knownLoadBalancers := sets.NewString()
	for _, n := range names {
		knownLoadBalancers.Insert(l.namer.LoadBalancer(n))
	}
	pool := l.snapshotter.Snapshot()

	// Delete unknown loadbalancers
	for name := range pool {
		if knownLoadBalancers.Has(name) {
			continue
		}
		glog.V(2).Infof("GCing loadbalancer %v", name)
		if err := l.Delete(name); err != nil {
			return err
		}
	}

	return nil
}

// Shutdown logs whether or not the pool is empty.
func (l *L7s) Shutdown() error {
	if err := l.GC([]string{}); err != nil {
		return err
	}
	glog.V(2).Infof("Loadbalancer pool shutdown.")
	return nil
}
