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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Sink struct {
	AbstractSink `json:",inline" protobuf:"bytes,1,opt,name=abstractSink"`
	// Fallback sink can be imagined as DLQ for primary Sink. The writes to Fallback sink will only be
	// initiated if the ud-sink response field sets it.
	// +optional
	Fallback *AbstractSink `json:"fallback,omitempty" protobuf:"bytes,2,opt,name=fallback"`
	// RetryStrategy struct encapsulates the settings for retrying operations in the event of failures.
	// +optional
	RetryStrategy *RetryStrategy `json:"retryStrategy,omitempty" protobuf:"bytes,3,opt,name=retryStrategy"`
}

type AbstractSink struct {
	// Log sink is used to write the data to the log.
	// +optional
	Log *Log `json:"log,omitempty" protobuf:"bytes,1,opt,name=log"`
	// Kafka sink is used to write the data to the Kafka.
	// +optional
	Kafka *KafkaSink `json:"kafka,omitempty" protobuf:"bytes,2,opt,name=kafka"`
	// Blackhole sink is used to write the data to the blackhole sink,
	// which is a sink that discards all the data written to it.
	// +optional
	Blackhole *Blackhole `json:"blackhole,omitempty" protobuf:"bytes,3,opt,name=blackhole"`
	// UDSink sink is used to write the data to the user-defined sink.
	// +optional
	UDSink *UDSink `json:"udsink,omitempty" protobuf:"bytes,4,opt,name=udsink"`
}

func (s Sink) getContainers(req getContainerReq) ([]corev1.Container, error) {
	containers := []corev1.Container{
		s.getMainContainer(req),
	}
	if s.UDSink != nil {
		containers = append(containers, s.getUDSinkContainer(req))
	}
	if s.Fallback != nil && s.Fallback.UDSink != nil {
		containers = append(containers, s.getFallbackUDSinkContainer(req))
	}
	return containers, nil
}

func (s Sink) getMainContainer(req getContainerReq) corev1.Container {
	return containerBuilder{}.init(req).args("processor", "--type="+string(VertexTypeSink), "--isbsvc-type="+string(req.isbSvcType)).build()
}

func (s Sink) getUDSinkContainer(mainContainerReq getContainerReq) corev1.Container {
	c := containerBuilder{}.
		name(CtrUdsink).
		imagePullPolicy(mainContainerReq.imagePullPolicy). // Use the same image pull policy as the main container
		appendVolumeMounts(mainContainerReq.volumeMounts...)
	x := s.UDSink.Container
	c = c.image(x.Image)
	if len(x.Command) > 0 {
		c = c.command(x.Command...)
	}
	if len(x.Args) > 0 {
		c = c.args(x.Args...)
	}
	c = c.appendEnv(corev1.EnvVar{Name: EnvUDContainerType, Value: UDContainerSink})
	c = c.appendEnv(x.Env...).appendVolumeMounts(x.VolumeMounts...).resources(x.Resources).securityContext(x.SecurityContext).appendEnvFrom(x.EnvFrom...)
	if x.ImagePullPolicy != nil {
		c = c.imagePullPolicy(*x.ImagePullPolicy)
	}
	container := c.build()
	container.LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/sidecar-livez",
				Port:   intstr.FromInt32(VertexMetricsPort),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       60,
		TimeoutSeconds:      30,
	}
	return container
}

func (s Sink) getFallbackUDSinkContainer(mainContainerReq getContainerReq) corev1.Container {
	c := containerBuilder{}.
		name(CtrFallbackUdsink).
		imagePullPolicy(mainContainerReq.imagePullPolicy). // Use the same image pull policy as the main container
		appendVolumeMounts(mainContainerReq.volumeMounts...)
	x := s.Fallback.UDSink.Container
	c = c.image(x.Image)
	if len(x.Command) > 0 {
		c = c.command(x.Command...)
	}
	if len(x.Args) > 0 {
		c = c.args(x.Args...)
	}
	c = c.appendEnv(corev1.EnvVar{Name: EnvUDContainerType, Value: UDContainerFallbackSink})
	c = c.appendEnv(x.Env...).appendVolumeMounts(x.VolumeMounts...).resources(x.Resources).securityContext(x.SecurityContext).appendEnvFrom(x.EnvFrom...)
	if x.ImagePullPolicy != nil {
		c = c.imagePullPolicy(*x.ImagePullPolicy)
	}
	container := c.build()
	container.LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/sidecar-livez",
				Port:   intstr.FromInt32(VertexMetricsPort),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       60,
		TimeoutSeconds:      30,
	}
	return container
}

// IsAnySinkSpecified returns true if any sink is specified.
func (a *AbstractSink) IsAnySinkSpecified() bool {
	return a.Log != nil || a.Kafka != nil || a.Blackhole != nil || a.UDSink != nil
}

// GetRetryStrategy retrieves the currently configured retry strategy from the sink object.
// If no strategy is explicitly set, it uses a default strategy. It's capable of merging personalized
// retry settings from a defined strategy with the default ones where some components have not been specified.
func (s *Sink) GetRetryStrategy() *RetryStrategy {
	// Obtains a default retry strategy which could be overridden by specific settings.
	retryStrategy := GetDefaultSinkRetryStrategy()

	// If no custom retry strategy is defined, return the default strategy.
	if s.RetryStrategy == nil {
		return retryStrategy
	}

	// If a custom back-off configuration is present, check and substitute the respective parts.
	if s.RetryStrategy.BackOff != nil {
		if s.RetryStrategy.BackOff.Interval != nil {
			retryStrategy.BackOff.Interval = s.RetryStrategy.BackOff.Interval
		}
		if s.RetryStrategy.BackOff.Steps != nil {
			retryStrategy.BackOff.Steps = s.RetryStrategy.BackOff.Steps
		}
	}

	// If a custom on-failure behavior is specified, override the default.
	if s.RetryStrategy.OnFailure != nil {
		retryStrategy.OnFailure = s.RetryStrategy.OnFailure
	}

	// Returns either the final retry strategy.
	return retryStrategy
}

// GetDefaultSinkRetryStrategy constructs and returns a default retry strategy with preset configurations.
func GetDefaultSinkRetryStrategy() *RetryStrategy {
	// Default number of retry steps and handling strategy on failure, globally defined.
	defaultRetrySteps := uint32(DefaultSinkRetrySteps)
	onFailure := DefaultSinkRetryStrategy

	// Assemble and return the default retry strategy encapsulating backoff mechanics and failure response.
	return &RetryStrategy{
		BackOff: &Backoff{
			// Default interval between retries.
			Interval: &metav1.Duration{Duration: DefaultSinkRetryInterval},
			// Default number of attempted retries.
			Steps: &defaultRetrySteps,
		},
		// Default action when all retries fail.
		OnFailure: &onFailure,
	}
}

// IsValidSinkRetryStrategy checks if the provided RetryStrategy is valid based on the sink's configuration.
// This validation ensures that the retry strategy is compatible with the sink's current setup
func (s *Sink) IsValidSinkRetryStrategy(strategy *RetryStrategy) error {
	// If the OnFailure strategy is set to fallback, but no fallback sink is provided in the Sink struct,
	// we return an error
	if *strategy.OnFailure == OnFailureFallback && s.Fallback == nil {
		return fmt.Errorf("given OnFailure strategy is fallback but fallback sink is not provided")
	}

	// If no errors are found, the function returns nil indicating the validation passed.
	return nil
}
