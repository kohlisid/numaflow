/*
 * Numaflow
 *
 * No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)
 *
 * The version of the OpenAPI document: latest
 * 
 * Generated by: https://openapi-generator.tech
 */

/// ServingStore : ServingStore to track and store data and metadata for tracking and serving.



#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct ServingStore {
    #[serde(rename = "ttl", skip_serializing_if = "Option::is_none")]
    pub ttl: Option<kube::core::Duration>,
    /// URL of the persistent store to write the callbacks
    #[serde(rename = "url")]
    pub url: String,
}

impl ServingStore {
    /// ServingStore to track and store data and metadata for tracking and serving.
    pub fn new(url: String) -> ServingStore {
        ServingStore {
            ttl: None,
            url,
        }
    }
}


