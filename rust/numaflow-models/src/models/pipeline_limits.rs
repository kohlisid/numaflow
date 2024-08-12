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
pub struct PipelineLimits {
    /// BufferMaxLength is used to define the max length of a buffer. Only applies to UDF and Source vertices as only they do buffer write. It can be overridden by the settings in vertex limits.
    #[serde(rename = "bufferMaxLength", skip_serializing_if = "Option::is_none")]
    pub buffer_max_length: Option<i64>,
    /// BufferUsageLimit is used to define the percentage of the buffer usage limit, a valid value should be less than 100, for example, 85. Only applies to UDF and Source vertices as only they do buffer write. It will be overridden by the settings in vertex limits.
    #[serde(rename = "bufferUsageLimit", skip_serializing_if = "Option::is_none")]
    pub buffer_usage_limit: Option<i64>,
    /// Read batch size for all the vertices in the pipeline, can be overridden by the vertex's limit settings.
    #[serde(rename = "readBatchSize", skip_serializing_if = "Option::is_none")]
    pub read_batch_size: Option<i64>,
    #[serde(rename = "readTimeout", skip_serializing_if = "Option::is_none")]
    pub read_timeout: Option<kube::core::Duration>,
}

impl PipelineLimits {
    pub fn new() -> PipelineLimits {
        PipelineLimits {
            buffer_max_length: None,
            buffer_usage_limit: None,
            read_batch_size: None,
            read_timeout: None,
        }
    }
}