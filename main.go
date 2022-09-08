package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/kubermatic/machine-controller/pkg/apis/cluster/v1alpha1"
	"k8c.io/kubermatic/v2/pkg/semver"
)

type KKPClient struct {
	// Seed client to create project, clusters and fetch kubeconfig of user clusters
	Seed ctrlruntimeclient.Client
	// User cluster client to deploy user's workload
	User ctrlruntimeclient.Client
}

// InitializeUserClusterClient gets a kubeconfig of user cluster and initialize its client
func (k *KKPClient) InitializeUserClusterClient(ctx context.Context, clusterID string) error {
	scheme := runtime.NewScheme()
	if err := clusterv1alpha1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return err
	}

	secret := corev1.Secret{}
	if err := wait.PollImmediate(5*time.Second, 3*time.Minute, func() (bool, error) {
		err := k.Seed.Get(
			ctx, ctrlruntimeclient.ObjectKey{
				Namespace: fmt.Sprintf("cluster-%s", clusterID),
				Name:      "admin-kubeconfig",
			},
			&secret,
		)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		return true, nil
	}); err != nil {
		return err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["kubeconfig"])
	if err != nil {
		return err
	}

	k.User, err = ctrlruntimeclient.New(restConfig, ctrlruntimeclient.Options{
		Scheme: scheme,
	})
	if err != nil {
		return err
	}

	return nil
}

func NewKKPClient(seedKubeConfig []byte) (*KKPClient, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(seedKubeConfig)
	if err != nil {
		return nil, err
	}

	seedClient, err := ctrlruntimeclient.New(restConfig, ctrlruntimeclient.Options{})
	if err != nil {
		return nil, err
	}

	return &KKPClient{Seed: seedClient}, nil
}

func main() {
	var (
		seedKubeconfigPath    string
		gcpServiceAccountPath string
		gcpNetwork            string
		gcpSubnet             string
		projectName           string
		clusterName           string
		machineName           string
		k8sVersion            string
	)

	flag.StringVar(&seedKubeconfigPath, "seed-kubeconfig", "", "path to seed kubeconfig")
	flag.StringVar(&gcpServiceAccountPath, "gcp-service-account", "", "GCP service account")
	flag.StringVar(&gcpNetwork, "gcp-network", "", "GCP network")
	flag.StringVar(&gcpSubnet, "gcp-subnet", "", "GCP subnet")
	flag.StringVar(&projectName, "project-name", "test-project", "Kubermatic project name")
	flag.StringVar(&clusterName, "cluster-name", "test-cluster", "Kubermatic cluster name")
	flag.StringVar(&machineName, "machine-name", "test-machine", "Kubermatic Machine Deployment name")
	flag.StringVar(&k8sVersion, "k8s-version", "1.23.9", "k8s version")
	flag.Parse()

	if seedKubeconfigPath == "" {
		fmt.Println("Seed kubeconfig is required")
		os.Exit(1)
	}
	if gcpServiceAccountPath == "" {
		fmt.Println("GCP service account is required")
		os.Exit(1)
	}

	seedKubeconfig, err := os.ReadFile(seedKubeconfigPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gcpServiceAccount, err := os.ReadFile(gcpServiceAccountPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	k8sSemver := semver.NewSemverOrDie(k8sVersion)
	kkpClient, err := NewKKPClient(seedKubeconfig)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ctx := context.TODO()

	// Create a project
	project, err := CreateProject(ctx, kkpClient.Seed, projectName)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create a user cluster
	// NOTE: projectName is a human-readable name, project.Name is an ID
	cluster, err := CreateCluster(ctx, kkpClient.Seed, clusterName, project.Name, string(gcpServiceAccount), gcpNetwork, gcpSubnet, *k8sSemver)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Initialize a user cluster client
	// NOTE: clusterName is a human-readable name, cluster.Name is an ID
	if err = kkpClient.InitializeUserClusterClient(ctx, cluster.Name); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create a user cluster nodes
	if err = CreateMachineDeployment(ctx, kkpClient.User, machineName, gcpNetwork, gcpSubnet, cluster); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Create a sample workload
	if err = CreateSamplePod(ctx, kkpClient.User); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
