/*
Copyright 2022 The Numaproj Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OnFailRetryStrategy string

// Constants representing the possible actions that can be taken when a failure occurs during an operation.
const (
	OnFailRetry    OnFailRetryStrategy = "retry"    // Retry the operation.
	OnFailFallback OnFailRetryStrategy = "fallback" // Reroute the operation to a fallback mechanism.
	OnFailDrop     OnFailRetryStrategy = "drop"     // Drop the operation and perform no further action.
)

// RetryStrategy struct encapsulates the settings for retrying operations in the event of failures.
// It includes a BackOff strategy to manage the timing of retries and defines the action to take upon failure.
type RetryStrategy struct {
	// BackOff specifies the parameters for the backoff strategy, controlling how delays between retries should increase.
	// +optional
	BackOff *Backoff `json:"backoff,omitempty" protobuf:"bytes,1,opt,name=backoff"`
	// OnFailure specifies the action to take when a retry fails. The default action is to retry.
	// +optional
	// +kubebuilder:default="retry"
	OnFailure OnFailRetryStrategy `json:"onFailure,omitempty" protobuf:"bytes,2,opt,name=onFailure"`
}

// Backoff defines parameters for a backoff retry strategy, used to systematically increase the delay between retries.
type Backoff struct {
	// Duration sets the initial delay before the first retry, after a failure occurs.
	// +kubebuilder:default="1ms"
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty" protobuf:"bytes,1,opt,name=duration"`
	// Factor determines how much the duration increases after each retry iteration.
	// The delay before the subsequent retry will be the previous delay multiplied by this factor.
	// Duration is multiplied by factor each iteration, if factor is not zero
	// and the limits imposed by Steps and Cap have not been reached.
	// Should not be negative.
	// The jitter does not contribute to the updates to the duration parameter.
	// +optional
	Factor Amount `json:"factor,omitempty" protobuf:"bytes,2,opt,name=factor"`
	// The sleep at each iteration is the duration plus an additional
	// amount chosen uniformly at random from the interval between
	// zero and `jitter*duration`.
	// +optional
	Jitter *Amount `json:"jitter,omitempty" protobuf:"bytes,3,opt,name=jitter"`
	// The remaining number of iterations in which the duration
	// parameter may change (but progress can be stopped earlier by
	// hitting the cap). If not positive, the duration is not
	// changed. Used for exponential backoff in combination with
	// Factor and Cap.
	// +optional
	// +kubebuilder:default=0
	Steps *uint32 `json:"steps,omitempty" protobuf:"bytes,4,opt,name=steps"`
	// A limit on revised values of the duration parameter. If a
	// multiplication by the factor parameter would make the duration
	// exceed the cap then the duration is set to the cap and the
	// steps parameter is set to zero.
	// Note: When the duration hits the cap, no further retries would occur
	// +optional
	Cap *metav1.Duration `json:"cap,omitempty" protobuf:"bytes,5,opt,name=cap"`
}
