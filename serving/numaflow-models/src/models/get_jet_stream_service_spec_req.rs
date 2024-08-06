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
pub struct GetJetStreamServiceSpecReq {
    #[serde(rename = "ClientPort")]
    pub client_port: i32,
    #[serde(rename = "ClusterPort")]
    pub cluster_port: i32,
    #[serde(rename = "Labels")]
    pub labels: ::std::collections::HashMap<String, String>,
    #[serde(rename = "MetricsPort")]
    pub metrics_port: i32,
    #[serde(rename = "MonitorPort")]
    pub monitor_port: i32,
}

impl GetJetStreamServiceSpecReq {
    pub fn new(
        client_port: i32,
        cluster_port: i32,
        labels: ::std::collections::HashMap<String, String>,
        metrics_port: i32,
        monitor_port: i32,
    ) -> GetJetStreamServiceSpecReq {
        GetJetStreamServiceSpecReq {
            client_port,
            cluster_port,
            labels,
            metrics_port,
            monitor_port,
        }
    }
}
