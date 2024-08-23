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

package forward

import (
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dfv1 "github.com/numaproj/numaflow/pkg/apis/numaflow/v1alpha1"
)

const (
	// The DefaultRetryFactor sets the default factor by which the retry interval duration will be multiplied
	// in successive retry attempts. It is set to 1, indicating no multiplication of the interval.
	DefaultRetryFactor = float64(1)
	// DefaultRetryJitter represents the randomness added to the delay intervals in retry attempts to prevent
	// synchronization issues. Setting this to zero means no jitter is applied.
	DefaultRetryJitter = float64(0)
	// DefaultRetrySteps sets the maximum number of retry attempts. It uses the maximum value for uint32,
	// effectively setting an extremely high limit for the retry count mimicking infinite.
	DefaultRetrySteps = uint32(math.MaxUint32)
	// DefaultRetrySleepInterval specifies the initial sleeping time interval for retries, set to 1 millisecond.
	DefaultRetrySleepInterval = 1 * time.Millisecond
)

// DefaultRetryDuration wraps the DefaultRetrySleepInterval into a Duration struct for easy use in configurations.
var DefaultRetryDuration = &metav1.Duration{Duration: DefaultRetrySleepInterval}

// defaultRetryStrategy constructs and returns a pointer to a RetryStrategy object initialized with default values.
// The retry strategy uses exponential backoff method which involves a constant retry duration, no jitter,
// and theoretically infinite steps owing to the DefaultRetrySteps setting.
// This mimics a fixed infinite retry strategy
func defaultRetryStrategy() *dfv1.RetryStrategy {
	factor := dfv1.FromFloat64(DefaultRetryFactor)
	jitter := dfv1.FromFloat64(DefaultRetryJitter)
	steps := DefaultRetrySteps
	return &dfv1.RetryStrategy{
		BackOff: &dfv1.Backoff{
			Duration: DefaultRetryDuration,
			Factor:   &factor,
			Jitter:   &jitter,
			Steps:    &steps,
		},
		OnFailure: dfv1.OnFailRetry,
	}
}
