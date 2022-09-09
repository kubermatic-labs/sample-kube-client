package main

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
	kubermaticv1helper "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1/helper"
	"k8c.io/kubermatic/v2/pkg/resources/reconciling"
	"k8c.io/kubermatic/v2/pkg/semver"
	kubermaticversions "k8c.io/kubermatic/v2/pkg/version/kubermatic"
)

// CreateCluster creates a user cluster
func CreateCluster(ctx context.Context, client ctrlruntimeclient.Client, projectName, serviceAccount, network, subnetwork string,
	k8sVersion semver.Semver) (*kubermaticv1.Cluster, error) {

	cluster := &kubermaticv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: utilrand.String(10),
			Labels: map[string]string{
				kubermaticv1.ProjectIDLabelKey: projectName,
			},
		},
		Spec: kubermaticv1.ClusterSpec{
			Version:           k8sVersion,
			HumanReadableName: "sample-cluster",
			Cloud: kubermaticv1.CloudSpec{
				GCP: &kubermaticv1.GCPCloudSpec{
					ServiceAccount: serviceAccount,
					Network:        network,
					Subnetwork:     subnetwork,
				},
				DatacenterName: "gcp-westeurope-2",
			},
			AuditLogging: &kubermaticv1.AuditLoggingSettings{
				Enabled:      false,
				PolicyPreset: "",
			},
			ClusterNetwork: kubermaticv1.ClusterNetworkingConfig{
				KonnectivityEnabled: pointer.Bool(true),
				IPFamily:            kubermaticv1.IPFamilyIPv4,
				ProxyMode:           "ipvs",
				Pods: kubermaticv1.NetworkRanges{
					CIDRBlocks: []string{"172.25.0.0/16"},
				},
				Services: kubermaticv1.NetworkRanges{
					CIDRBlocks: []string{"10.240.16.0/20"},
				},
				NodeCIDRMaskSizeIPv4:     pointer.Int32(24),
				NodeLocalDNSCacheEnabled: pointer.Bool(true),
			},
			CNIPlugin: &kubermaticv1.CNIPluginSettings{
				Type:    kubermaticv1.CNIPluginTypeCanal,
				Version: "v3.23",
			},
			OPAIntegration: &kubermaticv1.OPAIntegrationSettings{},
			MLA:            &kubermaticv1.MLASettings{},
			KubernetesDashboard: &kubermaticv1.KubernetesDashboard{
				Enabled: true,
			},
			EnableUserSSHKeyAgent:        pointer.Bool(true),
			EnableOperatingSystemManager: pointer.Bool(true),
			ContainerRuntime:             "containerd",
			Pause:                        false,
		},
	}

	err := client.Create(ctx, cluster)
	if err != nil {
		return nil, err
	}

	waiter := reconciling.WaitUntilObjectExistsInCacheConditionFunc(ctx, client, zap.NewNop().Sugar(), ctrlruntimeclient.ObjectKeyFromObject(cluster), cluster)
	if err := wait.Poll(100*time.Millisecond, 5*time.Second, func() (bool, error) {
		success, err := waiter()
		if err != nil {
			return false, err
		}
		if !success {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return nil, fmt.Errorf("failed waiting for the new cluster to appear in the cache: %w", err)
	}

	// In the future, this will not be required anymore, until then we sadly have
	// to manually ensure that the owner email is set correctly
	err = kubermaticv1helper.UpdateClusterStatus(ctx, client, cluster, func(c *kubermaticv1.Cluster) {
		c.Status.UserEmail = "marcin.franczyk@kubermatic.com"
	})
	if err != nil {
		return nil, err
	}

	// Wait for the cluster to be ready
	if err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		if err := client.Get(ctx, ctrlruntimeclient.ObjectKeyFromObject(cluster), cluster); err != nil {
			return false, fmt.Errorf("failed to get a project %w", err)
		}

		// Check if cluster is ready for interaction
		versions := kubermaticversions.NewDefaultVersions()
		if !kubermaticv1helper.IsClusterInitialized(cluster, versions) {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return nil, err
	}

	return cluster, nil
}
