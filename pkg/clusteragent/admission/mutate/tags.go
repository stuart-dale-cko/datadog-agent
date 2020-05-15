// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-2020 Datadog, Inc.

// +build kubeapiserver

package mutate

import (
	admiv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// InjectTags adds the DD_ENV, DD_VERSION, DD_SERVICE env vars to
// the pod template from pod and higher-level resource labels
func InjectTags(req *admiv1beta1.AdmissionRequest) (*admiv1beta1.AdmissionResponse, error) {
	return mutate(req, injectTags)
}

// injectTags injects DD_ENV, DD_VERSION, DD_SERVICE
// env vars into a pod template if needed
func injectTags(pod *corev1.Pod) error {
	// TODO: implement me
	return nil
}
