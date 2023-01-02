package actions

import (
	"context"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		containers := []corev1.EphemeralContainer{
			{
				EphemeralContainerCommon: corev1.EphemeralContainerCommon{
					// TODO: if d.spec.Name is empty or nil generate a name, is there a util for that?
					Name:                     d.spec.Name,
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
		_, err = d.corev1Client.
			Pods(podName.Namespace).
			UpdateEphemeralContainers(context.TODO(), podName.Name, pod, metav1.UpdateOptions{})

		if err != nil {
			return err
		}
	}

	return nil
}
