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

// Code generated by Openapi Generator. DO NOT EDIT.

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct MonoVertexLifecycle {
    /// DesiredPhase used to bring the MonoVertex from current phase to desired phase
    #[serde(rename = "desiredPhase", skip_serializing_if = "Option::is_none")]
    pub desired_phase: Option<String>,
}

impl MonoVertexLifecycle {
    pub fn new() -> MonoVertexLifecycle {
        MonoVertexLifecycle {
            desired_phase: None,
        }
    }
}
