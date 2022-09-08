package main

import (
	"context"
	"fmt"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
	kubermaticv1helper "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1/helper"
	"k8c.io/kubermatic/v2/pkg/resources/reconciling"
	"k8c.io/kubermatic/v2/pkg/semver"
)

// CreateCluster creates a user cluster
func CreateCluster(ctx context.Context, client ctrlruntimeclient.Client, name, projectName, serviceAccount, network, subnetwork string,
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
			HumanReadableName: name,
			Cloud: kubermaticv1.CloudSpec{
				GCP: &kubermaticv1.GCPCloudSpec{
					ServiceAccount: serviceAccount,
					Network:        network,
					Subnetwork:     subnetwork,
				},
				DatacenterName: "gcp-westeurope",
			},
			EnableOperatingSystemManager: pointer.Bool(true),
			ClusterNetwork: kubermaticv1.ClusterNetworkingConfig{
				KonnectivityEnabled: pointer.Bool(true),
			},
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

	// Wait for k8s etcd and API to be ready
	if err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		if err := client.Get(ctx, ctrlruntimeclient.ObjectKeyFromObject(cluster), cluster); err != nil {
			return false, fmt.Errorf("failed to get a project %w", err)
		}

		if cluster.Status.Phase != kubermaticv1.ClusterRunning {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return nil, err
	}

	return cluster, nil
}
