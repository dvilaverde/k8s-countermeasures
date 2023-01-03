package actions

import (
	"context"
	"fmt"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rand "k8s.io/apimachinery/pkg/util/rand"
	clientCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Debug struct {
	BaseAction
	corev1Client clientCoreV1.CoreV1Interface
	spec         v1alpha1.DebugSpec
}

func NewDebugAction(coreV1Client clientCoreV1.CoreV1Interface,
	client client.Client,
	spec v1alpha1.DebugSpec) *Debug {
	return NewDebugFromBase(BaseAction{
		client: client,
	}, coreV1Client, spec)
}

func NewDebugFromBase(base BaseAction,
	coreV1Client clientCoreV1.CoreV1Interface,
	spec v1alpha1.DebugSpec) *Debug {
	return &Debug{
		BaseAction:   base,
		spec:         spec,
		corev1Client: coreV1Client,
	}
}

func (d *Debug) GetTargetObjectName(data ActionData) string {
	return d.createObjectName("pod", d.spec.PodRef.Namespace, d.spec.PodRef.Name, data)
}

func (d *Debug) Perform(ctx context.Context, actionData ActionData) error {
	targetPod := d.spec.PodRef
	podName := ObjectKeyFromTemplate(targetPod.Namespace, targetPod.Name, actionData)
	targetContainerName := evaluateTemplate(targetPod.Container, actionData)

	pod := &corev1.Pod{}
	err := d.client.Get(ctx, podName, pod)
	if err != nil {
		return err
	}

	ephemeral := pod.Spec.EphemeralContainers
	addDebugContainer := true

	if len(ephemeral) > 0 {
		// don't install the container if there is already one installed
		for _, ec := range ephemeral {
			if ec.Name == d.spec.Name {
				addDebugContainer = false
			}
		}
	}

	if addDebugContainer {
		// Ephemeral Container Name is requried, so generate one if not provided
		name := d.spec.Name
		if len(name) == 0 {
			name = fmt.Sprintf("debug-%s", rand.String(5))
		}

		containers := []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					Name:                     name,
					Image:                    d.spec.Image,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Command:                  d.spec.Command,
					Args:                     d.spec.Args,
					Stdin:                    d.spec.StdIn,
					TTY:                      d.spec.TTY,
					TerminationMessagePolicy: "File",
				},
				TargetContainerName: targetContainerName,
			},
		}

		pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, containers...)

		opts := metav1.UpdateOptions{}
		if d.DryRun {
			opts.DryRun = []string{metav1.DryRunAll}
		}

		_, err = d.corev1Client.
			Pods(podName.Namespace).
			UpdateEphemeralContainers(ctx, podName.Name, pod, opts)
	}

	return err
}
