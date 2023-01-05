package actions

import (
	"context"
	"errors"
	"testing"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/dvilaverde/k8s-countermeasures/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPatch_Perform(t *testing.T) {

	deployment, err := runAction(false)
	if err != nil {
		t.Error(err)
	}

	meta := deployment.Spec.Template.ObjectMeta

	annotationValue, ok := meta.Annotations["operator.vilaverde.rocks/restarted"]
	assert.True(t, ok, "should have annotation")
	assert.True(t, len(meta.Annotations["operator.vilaverde.rocks/restarted"]) > 0, "should have true value")
	assert.Equal(t, "true", annotationValue)
}

func TestPatch_PerformDryRun(t *testing.T) {

	deployment, err := runAction(true)
	if err != nil {
		t.Error(err)
	}

	meta := deployment.Spec.Template.ObjectMeta

	annotationValue, ok := meta.Annotations["operator.vilaverde.rocks/restarted"]
	assert.False(t, ok, "should not have annotation")
	assert.Equal(t, "", annotationValue)
}

func runAction(dryRun bool) (*v1.Deployment, error) {
	deployment := &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: v1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "img1",
							Image: "image:latest",
						},
					},
				},
			},
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{deployment}

	// Create a fake client to mock API calls.
	k8sClient := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

	opt := client.MatchingLabels(map[string]string{"app": "test-app"})
	deploymentList := &v1.DeploymentList{}
	err := k8sClient.List(context.TODO(), deploymentList, opt)
	if err != nil {
		return nil, err
	}

	if len(deploymentList.Items) == 0 {
		return nil, errors.New("expected at least 1 deployment in the list")
	}

	spec := v1alpha1.PatchSpec{
		TargetObjectRef: v1alpha1.ObjectReference{
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  DeploymentNamespace,
			Name:       DeploymentName,
		},
		PatchType: types.MergePatchType,
		YAMLTemplate: `spec:
  template:
    metadata:
      annotations:
        operator.vilaverde.rocks/restarted: "true"`,
	}

	patch := NewPatchAction(k8sClient, spec)
	patch.DryRun = dryRun
	_, err = patch.Perform(context.TODO(), ActionData{
		Labels: make(map[string]string),
	})

	if err != nil {
		return nil, err
	}

	deployment = &v1.Deployment{}
	deploymentKey := types.NamespacedName{Namespace: DeploymentNamespace, Name: DeploymentName}

	err = k8sClient.Get(context.TODO(), deploymentKey, deployment)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func TestPatch_createPatch(t *testing.T) {
	deployment := &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: v1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "img1",
							Image: "image:latest",
							Env: []corev1.EnvVar{
								{
									Name:  "ENV.0",
									Value: "value0",
								},
								{
									Name:  "ENV.1",
									Value: "value1",
								},
								{
									Name:  "ENV.2",
									Value: "value2",
								},
							},
						},
					},
				},
			},
		},
	}

	spec := v1alpha1.PatchSpec{
		TargetObjectRef: v1alpha1.ObjectReference{
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  DeploymentNamespace,
			Name:       DeploymentName,
		},
		PatchType: types.StrategicMergePatchType,
		YAMLTemplate: `spec:
  template:
    spec: 
      containers:
        - name: img1
          env:
          - name: ENV.0
            value: {{- range (index .Object.spec.template.spec.containers 0).env }} {{ if eq .name "ENV.1" }} {{ .value }} {{ end -}} {{ end }}
          - name: ENV.1
            value: {{- range (index .Object.spec.template.spec.containers 0).env }} {{ if eq .name "ENV.0" }} {{ .value }} {{ end -}} {{ end }}`,
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{deployment}

	// Create a fake client to mock API calls.
	k8sClient := fake.NewClientBuilder().WithRuntimeObjects(objs...).Build()

	gvk, err := spec.TargetObjectRef.ToGroupVersionKind()
	if err != nil {
		t.Error(err)
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	objectName := ObjectKeyFromTemplate(PodNamespace, PodName, ActionData{})
	err = k8sClient.Get(context.TODO(), objectName, object)
	if err != nil {
		t.Error(err)
	}

	patch := NewPatchAction(k8sClient, spec)
	pd, err := patch.createPatch(PatchData{
		Unstructured: object,
	})

	if err != nil {
		t.Error(err)
	}
	bytes, _ := pd.Data(object)

	deploymentPatch := &v1.Deployment{}
	err = yaml.Unmarshal(bytes, &deploymentPatch)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, deploymentPatch)
	env := deploymentPatch.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, "ENV.0", env[0].Name)
	assert.Equal(t, "value1", env[0].Value)
	assert.Equal(t, "ENV.1", env[1].Name)
	assert.Equal(t, "value0", env[1].Value)
}
