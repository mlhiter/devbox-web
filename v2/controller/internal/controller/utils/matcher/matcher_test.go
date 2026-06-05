// Copyright © 2024 sealos.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matcher

import (
	"testing"
	"time"

	utilsresource "github.com/sealos-apps/devbox/v2/controller/internal/controller/utils/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodMatchExpectations(t *testing.T) {
	expectPod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					Env: []corev1.EnvVar{
						{Name: "ENV_VAR_1", Value: "value1"},
						{Name: "ENV_VAR_2", Value: "value2"},
					},
					Ports: []corev1.ContainerPort{
						{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
						{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "consistent pod",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ENV_VAR_1", Value: "value1"},
								{Name: "ENV_VAR_2", Value: "value2"},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "consistent pod with extra environment variable",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ENV_VAR_1", Value: "value1"},
								{Name: "ENV_VAR_2", Value: "value2"},
								{Name: "INJECTED_ENV", Value: "ignored"},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "inconsistent CPU",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ENV_VAR_1", Value: "value1"},
								{Name: "ENV_VAR_2", Value: "value2"},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "inconsistent environment variable",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ENV_VAR_1", Value: "value1"},
								{Name: "ENV_VAR_3", Value: "value3"},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "inconsistent port",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							Env: []corev1.EnvVar{
								{Name: "ENV_VAR_1", Value: "value1"},
								{Name: "ENV_VAR_2", Value: "value2"},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 9091, Protocol: corev1.ProtocolTCP},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	matchers := []PodMatcher{
		ResourceMatcher{},
		EnvVarMatcher{},
		PortMatcher{},
		EphemeralStorageMatcher{},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PodMatchExpectations(expectPod, tt.pod, matchers...)
			if result != tt.expected {
				t.Errorf("CheckPodConsistency() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestExtraResourceMatcher(t *testing.T) {
	expectPod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							utilsresource.GpuResourceName: resource.MustParse("1"),
						},
						Requests: corev1.ResourceList{
							utilsresource.GpuResourceName: resource.MustParse("1"),
						},
					},
				},
			},
		},
	}
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "consistent gpu",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									utilsresource.GpuResourceName: resource.MustParse("1"),
								},
								Requests: corev1.ResourceList{
									utilsresource.GpuResourceName: resource.MustParse("1"),
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: true,
		},
		{
			name: "inconsistent gpu count",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									utilsresource.GpuResourceName: resource.MustParse("2"),
								},
								Requests: corev1.ResourceList{
									utilsresource.GpuResourceName: resource.MustParse("2"),
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: false,
		},
		{
			name: "unexpected extra resource",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									utilsresource.GpuResourceName:           resource.MustParse("1"),
									corev1.ResourceName("example.com/fpga"): resource.MustParse("1"),
								},
								Requests: corev1.ResourceList{
									utilsresource.GpuResourceName: resource.MustParse("1"),
								},
							},
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{},
			},
			expected: false,
		},
	}

	extraMatcher := ExtraResourceMatcher{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extraMatcher.Match(expectPod, tt.pod)
			if result != tt.expected {
				t.Errorf("ExtraResourceMatcher.Match() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestExpectedAnnotationsMatcher(t *testing.T) {
	expectPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				utilsresource.GpuTypeAnnotation: "NVIDIA-Tesla P40",
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "consistent annotations",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utilsresource.GpuTypeAnnotation: "NVIDIA-Tesla P40",
					},
				},
			},
			expected: true,
		},
		{
			name: "missing expected annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: false,
		},
		{
			name: "different annotation value",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utilsresource.GpuTypeAnnotation: "NVIDIA-A100",
					},
				},
			},
			expected: false,
		},
		{
			name: "actual has extra annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						utilsresource.GpuTypeAnnotation: "NVIDIA-Tesla P40",
						"other":                         "ignored",
					},
				},
			},
			expected: true,
		},
	}

	annotationsMatcher := AnnotationsMatcher{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := annotationsMatcher.Match(expectPod, tt.pod)
			if result != tt.expected {
				t.Errorf("AnnotationsMatcher.Match() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestSchedulingMatcher(t *testing.T) {
	runtimeClassName := "devbox-runtime"
	expectPod := &corev1.Pod{
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				"accelerator": "nvidia",
			},
			RuntimeClassName: &runtimeClassName,
			Tolerations: []corev1.Toleration{
				{
					Key:      "gpu",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "consistent scheduling fields",
			pod: &corev1.Pod{
				Spec: expectPod.Spec,
			},
			expected: true,
		},
		{
			name: "treats default scheduler as equivalent when expected scheduler is empty",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations:      expectPod.Spec.Tolerations,
					SchedulerName:    corev1.DefaultSchedulerName,
				},
			},
			expected: true,
		},
		{
			name: "different scheduler name",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations:      expectPod.Spec.Tolerations,
					SchedulerName:    "gpu-scheduler",
				},
			},
			expected: false,
		},
		{
			name: "different node selector",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"accelerator": "amd",
					},
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations:      expectPod.Spec.Tolerations,
				},
			},
			expected: false,
		},
		{
			name: "missing toleration",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
				},
			},
			expected: false,
		},
		{
			name: "ignores controller injected required node pin",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations:      expectPod.Spec.Tolerations,
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchFields: []corev1.NodeSelectorRequirement{
											{
												Key:      "metadata.name",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"test-node"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "ignores kubernetes injected default tolerations",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations: append(
						append([]corev1.Toleration{}, expectPod.Spec.Tolerations...),
						corev1.Toleration{
							Key:               "node.kubernetes.io/not-ready",
							Operator:          corev1.TolerationOpExists,
							Effect:            corev1.TaintEffectNoExecute,
							TolerationSeconds: ptrDurationSeconds(5 * time.Minute),
						},
						corev1.Toleration{
							Key:               "node.kubernetes.io/unreachable",
							Operator:          corev1.TolerationOpExists,
							Effect:            corev1.TaintEffectNoExecute,
							TolerationSeconds: ptrDurationSeconds(5 * time.Minute),
						},
					),
				},
			},
			expected: true,
		},
		{
			name: "does not ignore changed user toleration",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations: []corev1.Toleration{
						{
							Key:      "gpu",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoExecute,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "keeps expected affinity while ignoring injected node pin",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeSelector:     expectPod.Spec.NodeSelector,
					RuntimeClassName: expectPod.Spec.RuntimeClassName,
					Tolerations:      expectPod.Spec.Tolerations,
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "disk",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"ssd"},
											},
										},
										MatchFields: []corev1.NodeSelectorRequirement{
											{
												Key:      "metadata.name",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"test-node"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	schedulingMatcher := SchedulingMatcher{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schedulingMatcher.Match(expectPod, tt.pod)
			if result != tt.expected {
				t.Errorf("SchedulingMatcher.Match() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func ptrDurationSeconds(duration time.Duration) *int64 {
	seconds := int64(duration / time.Second)
	return &seconds
}

func TestSchedulingMatcherWithExpectedAffinityAndInjectedNodePin(t *testing.T) {
	runtimeClassName := "devbox-runtime"
	expectPod := &corev1.Pod{
		Spec: corev1.PodSpec{
			RuntimeClassName: &runtimeClassName,
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "disk",
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"ssd"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	actualPod := expectPod.DeepCopy()
	actualPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.
		NodeSelectorTerms[0].MatchFields = []corev1.NodeSelectorRequirement{
		{
			Key:      "metadata.name",
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{"test-node"},
		},
	}

	schedulingMatcher := SchedulingMatcher{}
	if !schedulingMatcher.Match(expectPod, actualPod) {
		t.Fatal("SchedulingMatcher.Match() = false, expected true")
	}

	actualPod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.
		NodeSelectorTerms[0].MatchExpressions[0].Values = []string{"hdd"}
	if schedulingMatcher.Match(expectPod, actualPod) {
		t.Fatal("SchedulingMatcher.Match() = true, expected false")
	}
}
