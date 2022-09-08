package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateSamplePod(ctx context.Context, client ctrlruntimeclient.Client) error {
	samplePod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.14.2",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{},
	}

	return client.Create(ctx, &samplePod)
}
