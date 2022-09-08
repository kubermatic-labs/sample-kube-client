package main

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/kubermatic/machine-controller/pkg/apis/cluster/v1alpha1"
	gce "github.com/kubermatic/machine-controller/pkg/cloudprovider/provider/gce/types"
	providerconfig "github.com/kubermatic/machine-controller/pkg/providerconfig/types"
	apiv1 "k8c.io/kubermatic/v2/pkg/api/v1"
	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
)

// CreateMachineDeployment creates a user cluster node
func CreateMachineDeployment(ctx context.Context, client ctrlruntimeclient.Client, name, network, subnetwork string, cluster *kubermaticv1.Cluster) error {
	// GCE settings
	cloudConfig, err := json.Marshal(gce.CloudProviderSpec{
		Zone:                  providerconfig.ConfigVarString{Value: "europe-west2-a"},
		MachineType:           providerconfig.ConfigVarString{Value: "e2-highcpu-2"},
		DiskSize:              25,
		DiskType:              providerconfig.ConfigVarString{Value: "pd-standard"},
		Preemptible:           providerconfig.ConfigVarBool{Value: pointer.Bool(false)},
		Network:               providerconfig.ConfigVarString{Value: network},
		Subnetwork:            providerconfig.ConfigVarString{Value: subnetwork},
		AssignPublicIPAddress: &providerconfig.ConfigVarBool{Value: pointer.Bool(true)},
		MultiZone:             providerconfig.ConfigVarBool{Value: pointer.Bool(false)},
		Regional:              providerconfig.ConfigVarBool{Value: pointer.Bool(false)},
		Tags:                  []string{fmt.Sprintf("kubernetes-cluster-%s", cluster.Name)},
	})
	if err != nil {
		return err
	}

	// OS settings
	osConfig, err := json.Marshal(apiv1.OperatingSystemSpec{
		Ubuntu: &apiv1.UbuntuSpec{
			DistUpgradeOnBoot: false,
		},
	})
	if err != nil {
		return err
	}

	// Provider settings
	providerConfig, err := json.Marshal(providerconfig.Config{
		CloudProvider: "gce",
		CloudProviderSpec: runtime.RawExtension{
			Raw: cloudConfig,
		},
		OperatingSystem: "ubuntu",
		OperatingSystemSpec: runtime.RawExtension{
			Raw: osConfig,
		},
	})
	if err != nil {
		return err
	}

	machineDeployment := clusterv1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "kube-system",
		},
		Spec: clusterv1alpha1.MachineDeploymentSpec{
			Replicas: pointer.Int32(2),
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"machine": name,
				},
			},
			Template: clusterv1alpha1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"machine": name,
					},
				},
				Spec: clusterv1alpha1.MachineSpec{
					ProviderSpec: clusterv1alpha1.ProviderSpec{
						Value: &runtime.RawExtension{
							Raw: providerConfig,
						},
					},
					Versions: clusterv1alpha1.MachineVersionInfo{
						Kubelet: cluster.Spec.Version.String(),
					},
				},
			},
		},
	}

	if err := client.Create(ctx, &machineDeployment); err != nil {
		return err
	}

	return nil
}
