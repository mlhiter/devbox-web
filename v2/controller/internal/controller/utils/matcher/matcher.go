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
	"log/slog"
	"reflect"

	devboxv1alpha2 "github.com/sealos-apps/devbox/v2/controller/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

type PodMatcher interface {
	Match(expectPod, pod *corev1.Pod) bool
}

type resourceChecker func(expectPod, pod *corev1.Pod, expectContainer, container *corev1.Container) bool

func checkPodResources(expectPod, pod *corev1.Pod, checker resourceChecker) bool {
	if len(pod.Spec.Containers) == 0 {
		slog.Info("Pod has no containers")
		return false
	}
	if len(expectPod.Spec.Containers) == 0 {
		slog.Info("Expect pod has no containers")
		return false
	}
	return checker(expectPod, pod, &expectPod.Spec.Containers[0], &pod.Spec.Containers[0])
}

func isCommonResourceName(name corev1.ResourceName) bool {
	switch name {
	case corev1.ResourceCPU, corev1.ResourceMemory, corev1.ResourceEphemeralStorage:
		return true
	default:
		return false
	}
}

func compareExtraResourceList(expect corev1.ResourceList, actual corev1.ResourceList, listName string) bool {
	for name, expectQty := range expect {
		if isCommonResourceName(name) {
			continue
		}
		actualQty, ok := actual[name]
		if !ok {
			slog.Info("Extra resource missing", "list", listName, "resource", name)
			return false
		}
		if actualQty.Cmp(expectQty) != 0 {
			slog.Info("Extra resource not equal", "list", listName, "resource", name)
			return false
		}
	}
	for name := range actual {
		if isCommonResourceName(name) {
			continue
		}
		if _, ok := expect[name]; !ok {
			slog.Info("Unexpected extra resource", "list", listName, "resource", name)
			return false
		}
	}
	return true
}

func compareAnnotations(expect map[string]string, actual map[string]string) bool {
	if len(expect) == 0 {
		return true
	}
	for key, value := range expect {
		if actual == nil {
			slog.Info("Expected annotation missing", "annotation", key)
			return false
		}
		actualValue, ok := actual[key]
		if !ok || actualValue != value {
			slog.Info("Annotation is not equal", "annotation", key)
			return false
		}
	}
	return true
}

func schedulingTolerationsEqual(expect, actual []corev1.Toleration) bool {
	if reflect.DeepEqual(expect, actual) {
		return true
	}
	normalizedActual := stripInjectedDefaultTolerations(expect, actual)
	return reflect.DeepEqual(normalizeEmptyTolerations(expect), normalizeEmptyTolerations(normalizedActual))
}

func stripInjectedDefaultTolerations(
	expect []corev1.Toleration,
	actual []corev1.Toleration,
) []corev1.Toleration {
	normalized := make([]corev1.Toleration, 0, len(actual))
	usedExpectedDefaults := make([]bool, len(expect))
	for _, actualToleration := range actual {
		if !isDefaultNoExecuteToleration(actualToleration) {
			normalized = append(normalized, actualToleration)
			continue
		}
		if matchesExpectedDefaultToleration(
			actualToleration,
			expect,
			usedExpectedDefaults,
		) {
			normalized = append(normalized, actualToleration)
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func matchesExpectedDefaultToleration(
	actual corev1.Toleration,
	expect []corev1.Toleration,
	used []bool,
) bool {
	for i, expected := range expect {
		if used[i] ||
			!isDefaultNoExecuteToleration(expected) ||
			!reflect.DeepEqual(expected, actual) {
			continue
		}
		used[i] = true
		return true
	}
	return false
}

func isDefaultNoExecuteToleration(toleration corev1.Toleration) bool {
	if toleration.Operator != corev1.TolerationOpExists ||
		toleration.Effect != corev1.TaintEffectNoExecute ||
		toleration.Value != "" ||
		toleration.TolerationSeconds == nil ||
		*toleration.TolerationSeconds != 300 {
		return false
	}
	switch toleration.Key {
	case "node.kubernetes.io/not-ready", "node.kubernetes.io/unreachable":
		return true
	default:
		return false
	}
}

func normalizeEmptyTolerations(tolerations []corev1.Toleration) []corev1.Toleration {
	if len(tolerations) == 0 {
		return nil
	}
	return tolerations
}

func schedulingAffinityEqual(expect, actual *corev1.Affinity) bool {
	if reflect.DeepEqual(expect, actual) {
		return true
	}
	normalizedActual := stripInjectedNodeNameAffinity(expect, actual)
	return reflect.DeepEqual(normalizeEmptyAffinity(expect), normalizeEmptyAffinity(normalizedActual))
}

func stripInjectedNodeNameAffinity(
	expect, actual *corev1.Affinity,
) *corev1.Affinity {
	if actual == nil ||
		actual.NodeAffinity == nil ||
		actual.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return actual
	}

	normalized := actual.DeepCopy()
	actualRequired := normalized.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	expectTerms := []corev1.NodeSelectorTerm(nil)
	if expect != nil &&
		expect.NodeAffinity != nil &&
		expect.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		expectTerms = expect.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	}

	for termIndex := range actualRequired.NodeSelectorTerms {
		expectTerm := corev1.NodeSelectorTerm{}
		if termIndex < len(expectTerms) {
			expectTerm = expectTerms[termIndex]
		}
		actualTerm := &actualRequired.NodeSelectorTerms[termIndex]
		actualTerm.MatchFields = keepExpectedNodeNameFields(
			expectTerm.MatchFields,
			actualTerm.MatchFields,
		)
	}
	return normalized
}

func keepExpectedNodeNameFields(
	expectFields []corev1.NodeSelectorRequirement,
	actualFields []corev1.NodeSelectorRequirement,
) []corev1.NodeSelectorRequirement {
	keptFields := make([]corev1.NodeSelectorRequirement, 0, len(actualFields))
	usedExpectedNodeNameFields := make([]bool, len(expectFields))
	for _, actualField := range actualFields {
		if actualField.Key != "metadata.name" {
			keptFields = append(keptFields, actualField)
			continue
		}

		if matchesExpectedNodeNameField(actualField, expectFields, usedExpectedNodeNameFields) {
			keptFields = append(keptFields, actualField)
		}
	}
	if len(keptFields) == 0 {
		return nil
	}
	return keptFields
}

func matchesExpectedNodeNameField(
	actualField corev1.NodeSelectorRequirement,
	expectFields []corev1.NodeSelectorRequirement,
	used []bool,
) bool {
	for i, expectField := range expectFields {
		if used[i] ||
			expectField.Key != "metadata.name" ||
			!reflect.DeepEqual(expectField, actualField) {
			continue
		}
		used[i] = true
		return true
	}
	return false
}

func normalizeEmptyAffinity(affinity *corev1.Affinity) *corev1.Affinity {
	if affinity == nil {
		return nil
	}
	normalized := affinity.DeepCopy()
	if normalized.NodeAffinity != nil &&
		normalized.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		required := normalized.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		required.NodeSelectorTerms = pruneEmptyNodeSelectorTerms(required.NodeSelectorTerms)
		if len(required.NodeSelectorTerms) == 0 {
			normalized.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = nil
		}
	}
	if normalized.NodeAffinity != nil &&
		normalized.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil &&
		len(normalized.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution) == 0 {
		normalized.NodeAffinity = nil
	}
	if normalized.NodeAffinity == nil &&
		normalized.PodAffinity == nil &&
		normalized.PodAntiAffinity == nil {
		return nil
	}
	return normalized
}

func pruneEmptyNodeSelectorTerms(
	terms []corev1.NodeSelectorTerm,
) []corev1.NodeSelectorTerm {
	pruned := make([]corev1.NodeSelectorTerm, 0, len(terms))
	for _, term := range terms {
		if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
			continue
		}
		pruned = append(pruned, term)
	}
	return pruned
}

type ResourceMatcher struct{}

func (r ResourceMatcher) Match(expectPod, pod *corev1.Pod) bool {
	return checkPodResources(expectPod, pod, commonResourceChecker)
}

func commonResourceChecker(_, _ *corev1.Pod, expectContainer, container *corev1.Container) bool {
	if container.Resources.Requests.Cpu().Cmp(*expectContainer.Resources.Requests.Cpu()) != 0 {
		slog.Info("CPU requests are not equal")
		return false
	}
	if container.Resources.Limits.Cpu().Cmp(*expectContainer.Resources.Limits.Cpu()) != 0 {
		slog.Info("CPU limits are not equal")
		return false
	}
	if container.Resources.Requests.Memory().
		Cmp(*expectContainer.Resources.Requests.Memory()) !=
		0 {
		slog.Info("Memory requests are not equal")
		return false
	}
	if container.Resources.Limits.Memory().Cmp(*expectContainer.Resources.Limits.Memory()) != 0 {
		slog.Info("Memory limits are not equal")
		return false
	}
	return true
}

type ExtraResourceMatcher struct{}

func (m ExtraResourceMatcher) Match(expectPod, pod *corev1.Pod) bool {
	return checkPodResources(expectPod, pod, extraResourceChecker)
}

func extraResourceChecker(expectPod, pod *corev1.Pod, expectContainer, container *corev1.Container) bool {
	return compareExtraResourceList(expectContainer.Resources.Limits, container.Resources.Limits, "limits") &&
		compareExtraResourceList(expectContainer.Resources.Requests, container.Resources.Requests, "requests")
}

type AnnotationsMatcher struct{}

func (m AnnotationsMatcher) Match(expectPod, pod *corev1.Pod) bool {
	return compareAnnotations(expectPod.Annotations, pod.Annotations)
}

type SchedulingMatcher struct{}

func (m SchedulingMatcher) Match(expectPod, pod *corev1.Pod) bool {
	if !reflect.DeepEqual(expectPod.Spec.NodeSelector, pod.Spec.NodeSelector) {
		slog.Info("NodeSelector is not equal")
		return false
	}
	if !schedulingTolerationsEqual(expectPod.Spec.Tolerations, pod.Spec.Tolerations) {
		slog.Info("Tolerations are not equal")
		return false
	}
	if !schedulingAffinityEqual(expectPod.Spec.Affinity, pod.Spec.Affinity) {
		slog.Info("Affinity is not equal")
		return false
	}
	if !reflect.DeepEqual(expectPod.Spec.RuntimeClassName, pod.Spec.RuntimeClassName) {
		slog.Info("RuntimeClassName is not equal")
		return false
	}
	if !schedulerNameEqual(expectPod.Spec.SchedulerName, pod.Spec.SchedulerName) {
		slog.Info("SchedulerName is not equal")
		return false
	}
	return true
}

func schedulerNameEqual(expect, actual string) bool {
	if expect == actual {
		return true
	}
	return expect == "" && actual == corev1.DefaultSchedulerName
}

type EphemeralStorageMatcher struct{}

func (e EphemeralStorageMatcher) Match(expectPod, pod *corev1.Pod) bool {
	if len(pod.Spec.Containers) == 0 {
		slog.Info("Pod has no containers")
		return false
	}
	container := pod.Spec.Containers[0]
	expectContainer := expectPod.Spec.Containers[0]

	if container.Resources.Limits.StorageEphemeral().
		Cmp(*expectContainer.Resources.Limits.StorageEphemeral()) !=
		0 {
		slog.Info("Ephemeral-Storage limits are not equal")
		return false
	}
	if container.Resources.Requests.StorageEphemeral().
		Cmp(*expectContainer.Resources.Requests.StorageEphemeral()) !=
		0 {
		slog.Info("Ephemeral-Storage requests are not equal")
		return false
	}
	return true
}

type EnvVarMatcher struct{}

func (e EnvVarMatcher) Match(expectPod, pod *corev1.Pod) bool {
	if len(pod.Spec.Containers) == 0 {
		slog.Info("Pod has no containers")
		return false
	}
	container := pod.Spec.Containers[0]
	expectContainer := expectPod.Spec.Containers[0]

	for _, expectEnv := range expectContainer.Env {
		found := false
		for _, env := range container.Env {
			if expectEnv.Name == "SEALOS_COMMIT_IMAGE_NAME" && env.Name == expectEnv.Name {
				found = true
				break
			}
			if env.Name == expectEnv.Name && env.Value == expectEnv.Value {
				found = true
				break
			}
		}
		if !found {
			slog.Info(
				"Environment variables are not equal",
				"env not found",
				expectEnv.Name,
				"env value",
				expectEnv.Value,
			)
			return false
		}
	}
	return true
}

type PortMatcher struct{}

func (p PortMatcher) Match(expectPod, pod *corev1.Pod) bool {
	if len(pod.Spec.Containers) == 0 {
		slog.Info("Pod has no containers")
		return false
	}
	container := pod.Spec.Containers[0]
	expectContainer := expectPod.Spec.Containers[0]

	if len(container.Ports) != len(expectContainer.Ports) {
		slog.Info("Port count mismatch")
		return false
	}

	for _, expectPort := range expectContainer.Ports {
		found := false
		for _, podPort := range container.Ports {
			if expectPort.ContainerPort == podPort.ContainerPort &&
				expectPort.Protocol == podPort.Protocol {
				found = true
				break
			}
		}
		if !found {
			slog.Info("Ports are not equal")
			return false
		}
	}
	return true
}

type StorageLimitMatcher struct{}

func (s StorageLimitMatcher) Match(expectPod, pod *corev1.Pod) bool {
	return expectPod.Annotations[devboxv1alpha2.AnnotationStorageLimit] == pod.Annotations[devboxv1alpha2.AnnotationStorageLimit]
}

type InitAnnotationMatcher struct{}

func (m InitAnnotationMatcher) Match(expectPod, pod *corev1.Pod) bool {
	return expectPod.Annotations[devboxv1alpha2.AnnotationInit] == pod.Annotations[devboxv1alpha2.AnnotationInit]
}

// PredicateCommitStatus returns the commit status of the pod
// if the pod container id is empty, it means the pod is pending or has't started, we can assume the image has not been committed
// otherwise, it means the pod has been started, we can assume the image has been committed

func PodMatchExpectations(expectPod, pod *corev1.Pod, matchers ...PodMatcher) bool {
	for _, matcher := range matchers {
		if !matcher.Match(expectPod, pod) {
			return false
		}
	}
	return true
}
