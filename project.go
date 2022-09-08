package main

import (
	"context"
	"fmt"
	"time"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	kubermaticv1 "k8c.io/kubermatic/v2/pkg/apis/kubermatic/v1"
)

// CreateProject creates project on a seed cluster
func CreateProject(ctx context.Context, client ctrlruntimeclient.Client, name string) (*kubermaticv1.Project, error) {
	project := &kubermaticv1.Project{}
	project.Name = utilrand.String(10)
	project.Spec.Name = name

	if err := client.Create(ctx, project); err != nil {
		return nil, err
	}

	// Wait for project to be active
	if err := wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
		if err := client.Get(ctx, ctrlruntimeclient.ObjectKeyFromObject(project), project); err != nil {
			return false, fmt.Errorf("failed to get a project %w", err)
		}

		if project.Status.Phase != kubermaticv1.ProjectActive {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return nil, err
	}

	return project, nil
}
