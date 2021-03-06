// Copyright 2018 Tetrate, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datamodel

import (
	"net"
)

//go:generate mockgen -source=model.go -destination=mock/mock_datamodel.go

// DataModel is the standard interface that all concrete DataModel types will adhere to.
// Objects can be stored in any datastore (in mem, etcd, rdbms, etc.)
type DataModel interface {
	CreateGlobalService(g *GlobalService) error
	GetGlobalService(name string) (*GlobalService, error)
	UpdateGlobalService(g *GlobalService) error
	DeleteGlobalService(name string) (*GlobalService, error)
	ListGlobalServices() map[string]*GlobalService
}

// Port describes the properties of a specific port of a service.
type Port struct {
	// ServicePort is a valid non-negative integer port number. This is the port clients call in to.
	ServicePort uint32 `json:"service_port"`

	// Protocol exposed on the port.
	// MUST BE one of HTTP|HTTPS|GRPC|HTTP2|MONGO|TCP.
	Protocol string `json:"protocol"`

	// BackendPort is the corresponding port exposed by the backend services.
	BackendPort uint32 `json:"backend_port"`

	// Name associated with the port
	Name string `json:"name"`
}

// GlobalService is a service exposed from a cluster. All traffic will
// arrive at the ingress gateway of the cluster.
type GlobalService struct {
	// Name is a globally unique name to refer to this service in other API
	// calls. The same global service can be exposed from multiple clusters
	// in cases where the customer wants a global load balancing across
	// clusters.
	Name string `json:"name"`

	// DNSPrefixes for hosts used by the service.  The full DNS name will be
	// constructed based on the pre-configured DNS suffix. For example,
	// foo.ns1 will become foo.ns1.svc.cluster.global if svc.cluster.global
	// is the DNS suffix.
	DNSPrefixes []string `json:"dns_prefixes"`

	// Ports exposed by the service.
	Ports []Port `json:"ports"`

	// Backend services in different clusters
	Backends map[string]string `json:"backends"`

	// Address is the VIP assigned to this service
	Address net.IP `json:"address"`

	// Unregistered is set by the server to indicate that
	// the service will be removed in the future after cleaning up
	// the associated configurations from the respective clusters
	Unregistered bool `json:"unregistered,omitempty"`
}

// Cluster represents a cluster that can host services.
type Cluster struct {
	// Name of this cluster
	Name string `json:"name"`
	// Address is the DNS address of this cluster
	Address string `json:"address"`
}

// Clusters is a list of Cluster
type Clusters []Cluster

// Infrastructure abstracts the system that has information about
// the actual location of the gateways, their addresses, handles to
// the underlying clusters connected to this manager, etc.
type Infrastructure interface {
	// GetIngressGatewayAddress returns the address of the ingress gateway
	// of a cluster, that is accessible from other clusters.
	GetIngressGatewayAddress(clusterName string) (string, error)
}
