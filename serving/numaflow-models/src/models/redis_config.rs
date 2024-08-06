/*
 * Numaflow
 *
 * No description provided (generated by Openapi Generator https://github.com/openapitools/openapi-generator)
 *
 * The version of the OpenAPI document: latest
 * 
 * Generated by: https://openapi-generator.tech
 */




#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
pub struct RedisConfig {
    /// Only required when Sentinel is used
    #[serde(rename = "masterName", skip_serializing_if = "Option::is_none")]
    pub master_name: Option<String>,
    #[serde(rename = "password", skip_serializing_if = "Option::is_none")]
    pub password: Option<k8s_openapi::api::core::v1::SecretKeySelector>,
    #[serde(rename = "sentinelPassword", skip_serializing_if = "Option::is_none")]
    pub sentinel_password: Option<k8s_openapi::api::core::v1::SecretKeySelector>,
    /// Sentinel URL, will be ignored if Redis URL is provided
    #[serde(rename = "sentinelUrl", skip_serializing_if = "Option::is_none")]
    pub sentinel_url: Option<String>,
    /// Redis URL
    #[serde(rename = "url", skip_serializing_if = "Option::is_none")]
    pub url: Option<String>,
    /// Redis user
    #[serde(rename = "user", skip_serializing_if = "Option::is_none")]
    pub user: Option<String>,
}

impl RedisConfig {
    pub fn new() -> RedisConfig {
        RedisConfig {
            master_name: None,
            password: None,
            sentinel_password: None,
            sentinel_url: None,
            url: None,
            user: None,
        }
    }
}


