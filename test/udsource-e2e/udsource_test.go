//go:build test

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

package e2e

import (
	"testing"

	"github.com/stretchr/testify/suite"

	. "github.com/numaproj/numaflow/test/fixtures"
)

type UserDefinedSourceSuite struct {
	E2ESuite
}

// this test is currently broken because the UDSource is not completely implemented yet.
// I put it here to exercise the test-driven development process.
// TODO - include it in the CI workflow once we finish implementing UDSource
func (s *UserDefinedSourceSuite) TestSimpleSource() {
	w := s.Given().Pipeline("@testdata/simple-source.yaml").
		When().
		CreatePipelineAndWait()
	defer w.DeletePipelineAndWait()

	// wait for all the pods to come up
	w.Expect().VertexPodsRunning()

	// the user-defined simple source sends the read index of the message as the message content
	// verify the sink get the first batch of data
	w.Expect().SinkContains("out", "0")
	w.Expect().SinkContains("out", "1")
	// verify the sink get the second batch of data
	w.Expect().SinkContains("out", "2")
}

func TestUserDefinedSourceSuite(t *testing.T) {
	suite.Run(t, new(UserDefinedSourceSuite))
}