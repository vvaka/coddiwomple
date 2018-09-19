package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/user"
	"path/filepath"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/operator-framework/operator-sdk/pkg/sdk/metrics"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tetratelabs/mcc/pkg/datamodel/mem"
	"github.com/tetratelabs/mcc/pkg/ui"
)

const (
	resourcePluralName = "v1/services"
	allNamespaces      = ""
	resyncPeriod       = 5
)

var (
	groupVersionKind = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}
)

func uiCmd() (serve *cobra.Command) {
	var (
		port        int
		kubeConfigs []string
		//clusters    []string
		clustersFile string
	)

	serve = &cobra.Command{
		Use:     "ui",
		Short:   "Starts the mcc UI on localhost",
		Example: "mcc ui --port 123",
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterNames, clusters, infra, err := clustersFromFile(clustersFile)
			if err != nil {
				return errors.Wrap(err, "failed to read clusters file")
			}

			collector := metrics.New()
			metrics.RegisterCollector(collector)

			dm := mem.NewDataModel()
			for _, cluster := range clusters {
				client, err := k8sClientFor(cluster.KubeconfigPath, cluster.KubeconfigContext)
				if err != nil {
					return fmt.Errorf("failed to construct k8s client %q with context %q: %v", cluster.KubeconfigPath, cluster.KubeconfigContext, err)
				}
				log.Printf("Watching for %q across all namespaces in cluster %q with resync period %d",
					resourcePluralName, cluster.Name, resyncPeriod)
				i := sdk.NewInformerWithHandler(resourcePluralName, allNamespaces, client, resyncPeriod, collector, dm.Handler(cluster.Name))
				go i.Run(context.Background())
			}

			mux := http.NewServeMux()
			ui.RegisterHandlers(dm, infra, clusterNames, mux)
			address := fmt.Sprintf(":%d", port)
			log.Printf("starting server on %s", address)
			return http.ListenAndServe(address, mux)
		},
	}

	serve.PersistentFlags().IntVar(&port, "port", 8080, "the port to start the local UI server on")
	serve.PersistentFlags().StringArrayVar(&kubeConfigs, "kube-config", nil,
		"kubeconfig location in the form 'name:filepath:contextNameOne,contextNameTwo' where contextNameOne is the name passed to "+
			"`kubectl --context=contextNameOne`. If no contexts are provided the tool will use the default context. "+
			"This flag can be repeated to set multiple kubeconfigs with multiple contexts each. "+
			"The name should match the name for the cluster in the cluster-file.")
	//serve.PersistentFlags().StringArrayVar(&clusters, "cluster", []string{}, "The clusters that we'll generate configs for, "+
	//	"in the format name,port,endpoint1,endpoint2,..., e.g. --remote-cluster=remote,80,10.11.12.13. "+
	//	"This flag can be provided multiple times for multiple remote clusters. "+
	//	"For each cluster, the endpoints must be either all IP addresses or all DNS names, and not a mix of both. "+
	//	"The names for these clusters must match the names of the contexts in each .")

	serve.PersistentFlags().StringVar(&clustersFile, "cluster-file", "",
		`Path to a file with a JSON array of clusters, where a cluster is an object like '{"name": "ClusterName", "address": "dns.address.of.cluster"}'`)

	return serve
}

func k8sClientFor(path, context string) (dynamic.ResourceInterface, error) {
	if path == "" {
		path = "~/.kube/config"
	}

	absPath, err := expand(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to expand relative path %q", path)
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: absPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create config with: %v", err)
	}

	k, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client with: %v", err)
	}

	cachedDiscoveryClient := cached.NewMemCacheClient(k.Discovery())
	restMapper := discovery.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient, meta.InterfacesForUnstructured)
	restMapper.Reset()
	config.ContentConfig = dynamic.ContentConfig()
	clientPool := dynamic.NewClientPool(config, restMapper, dynamic.LegacyAPIPathResolverFunc)

	mapping, err := restMapper.RESTMapping(groupVersionKind.GroupKind(), groupVersionKind.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST mapping for service with: %v", err)
	}
	client, err := clientPool.ClientForGroupVersionKind(groupVersionKind)
	if err != nil {
		return nil, err
	}

	return client.Resource(&metav1.APIResource{
		Name:       mapping.Resource,
		Namespaced: mapping.Scope == meta.RESTScopeNamespace,
		Kind:       "Service",
	}, allNamespaces), nil
}

func expand(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}