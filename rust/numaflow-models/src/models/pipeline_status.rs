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
pub struct PipelineStatus {
    /// Conditions are the latest available observations of a resource's current state.
    #[serde(rename = "conditions", skip_serializing_if = "Option::is_none")]
    pub conditions: Option<Vec<k8s_openapi::apimachinery::pkg::apis::meta::v1::Condition>>,
    /// Field to indicate if a pipeline drain successfully occurred, or it timed out. Set to true when the Pipeline is in Paused state, and after it has successfully been drained. defaults to false
    #[serde(rename = "drainedOnPause", skip_serializing_if = "Option::is_none")]
    pub drained_on_pause: Option<bool>,
    #[serde(rename = "lastUpdated", skip_serializing_if = "Option::is_none")]
    pub last_updated: Option<k8s_openapi::apimachinery::pkg::apis::meta::v1::Time>,
    #[serde(rename = "mapUDFCount", skip_serializing_if = "Option::is_none")]
    pub map_udf_count: Option<i64>,
    #[serde(rename = "message", skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    /// The generation observed by the Pipeline controller.
    #[serde(rename = "observedGeneration", skip_serializing_if = "Option::is_none")]
    pub observed_generation: Option<i64>,
    #[serde(rename = "phase", skip_serializing_if = "Option::is_none")]
    pub phase: Option<String>,
    #[serde(rename = "reduceUDFCount", skip_serializing_if = "Option::is_none")]
    pub reduce_udf_count: Option<i64>,
    #[serde(rename = "sinkCount", skip_serializing_if = "Option::is_none")]
    pub sink_count: Option<i64>,
    #[serde(rename = "sourceCount", skip_serializing_if = "Option::is_none")]
    pub source_count: Option<i64>,
    #[serde(rename = "udfCount", skip_serializing_if = "Option::is_none")]
    pub udf_count: Option<i64>,
    #[serde(rename = "vertexCount", skip_serializing_if = "Option::is_none")]
    pub vertex_count: Option<i64>,
}

impl PipelineStatus {
    pub fn new() -> PipelineStatus {
        PipelineStatus {
            conditions: None,
            drained_on_pause: None,
            last_updated: None,
            map_udf_count: None,
            message: None,
            observed_generation: None,
            phase: None,
            reduce_udf_count: None,
            sink_count: None,
            source_count: None,
            udf_count: None,
            vertex_count: None,
        }
    }
}
