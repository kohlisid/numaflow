use std::error::Error;

use numaflow::sink::{self, Response, SinkRequest};
use reqwest::Client;
use tracing::{error, warn};

const NUMAFLOW_CALLBACK_URL_HEADER: &str = "X-Numaflow-Callback-Url";
const NUMAFLOW_ID_HEADER: &str = "X-Numaflow-Id";
const ENV_NUMAFLOW_CALLBACK_URL_KEY: &str = "NUMAFLOW_CALLBACK_URL_KEY";
const ENV_NUMAFLOW_MESSAGE_ID_KEY: &str = "NUMAFLOW_MESSAGE_ID_KEY";

/// servesink is a Numaflow Sink which forwards the payload to the Numaflow serving URL.
pub async fn servesink() -> Result<(), Box<dyn Error + Send + Sync>> {
    sink::Server::new(ServeSink::new()).start().await
}

struct ServeSink {
    callback_url_key: String,
    message_id_key: String,
    client: Client,
}

impl ServeSink {
    fn new() -> Self {
        // extract the callback url key from the environment
        let callback_url_key = std::env::var(ENV_NUMAFLOW_CALLBACK_URL_KEY)
            .unwrap_or_else(|_| NUMAFLOW_CALLBACK_URL_HEADER.to_string());

        // extract the message id key from the environment
        let message_id_key = std::env::var(ENV_NUMAFLOW_MESSAGE_ID_KEY)
            .unwrap_or_else(|_| NUMAFLOW_ID_HEADER.to_string());

        Self {
            callback_url_key,
            message_id_key,
            client: Client::builder()
                .danger_accept_invalid_certs(true)
                .build()
                .unwrap(),
        }
    }
}

#[tonic::async_trait]
impl sink::Sinker for ServeSink {
    async fn sink(&self, mut input: tokio::sync::mpsc::Receiver<SinkRequest>) -> Vec<Response> {
        let mut responses: Vec<Response> = Vec::new();

        while let Some(datum) = input.recv().await {
            // if the callback url is absent, ignore the request
            let url = match datum.headers.get(self.callback_url_key.as_str()) {
                Some(url) => url,
                None => {
                    warn!(
                        "Missing {} header, Ignoring the request",
                        self.callback_url_key
                    );
                    responses.push(Response::ok(datum.id));
                    continue;
                }
            };

            // if the numaflow id is absent, ignore the request
            let numaflow_id = match datum.headers.get(self.message_id_key.as_str()) {
                Some(id) => id,
                None => {
                    warn!(
                        "Missing {} header, Ignoring the request",
                        self.message_id_key
                    );
                    responses.push(Response::ok(datum.id));
                    continue;
                }
            };

            let resp = self
                .client
                .post(format!("{}_{}", url, "save"))
                .header(self.message_id_key.as_str(), numaflow_id)
                .header("id", numaflow_id)
                .body(datum.value)
                .send()
                .await;

            let response = match resp {
                Ok(_) => Response::ok(datum.id),
                Err(e) => {
                    error!("Sending result to serving URL {:?}", e);
                    Response::failure(datum.id, format!("Failed to send: {}", e))
                }
            };

            responses.push(response);
        }
        responses
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use numaflow::sink::{SinkRequest, Sinker};
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::TcpListener;
    use tokio::sync::mpsc;

    use super::*;

    #[tokio::test]
    async fn test_serve_sink_without_url_header() {
        let serve_sink = ServeSink::new();
        let (tx, rx) = mpsc::channel(1);

        let mut headers = HashMap::new();
        headers.insert(NUMAFLOW_ID_HEADER.to_string(), "12345".to_string());

        let request = SinkRequest {
            keys: vec![],
            id: "1".to_string(),
            value: b"test".to_vec(),
            watermark: Default::default(),
            headers,
            event_time: Default::default(),
        };

        tx.send(request).await.unwrap();
        drop(tx); // Close the sender to end the stream

        let responses = serve_sink.sink(rx).await;
        assert_eq!(responses.len(), 1);
        assert!(responses[0].success);
    }

    #[tokio::test]
    async fn test_serve_sink_without_id_header() {
        let serve_sink = ServeSink::new();
        let (tx, rx) = mpsc::channel(1);

        let mut headers = HashMap::new();
        headers.insert(
            NUMAFLOW_CALLBACK_URL_HEADER.to_string(),
            "http://localhost:8080".to_string(),
        );

        let request = SinkRequest {
            keys: vec![],
            id: "1".to_string(),
            value: b"test".to_vec(),
            watermark: Default::default(),
            headers,
            event_time: Default::default(),
        };

        tx.send(request).await.unwrap();
        drop(tx); // Close the sender to end the stream

        let responses = serve_sink.sink(rx).await;
        assert_eq!(responses.len(), 1);
        assert!(responses[0].success);
    }

    async fn start_server() -> (String, mpsc::Sender<()>) {
        let (shutdown_tx, mut shutdown_rx) = mpsc::channel(1);
        let listener = TcpListener::bind("0.0.0.0:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let addr_str = format!("{}", addr);
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    _ = shutdown_rx.recv() => {
                        break;
                    }
                    Ok((mut socket, _)) = listener.accept() => {
                        tokio::spawn(async move {
                            let mut buffer = [0; 1024];
                            let _ = socket.read(&mut buffer).await.unwrap();
                            let request = String::from_utf8_lossy(&buffer[..]);
                            let response = if request.contains("/error") {
                                "HTTP/1.1 500 INTERNAL SERVER ERROR\r\n\
                                content-length: 0\r\n\
                            \r\n"
                            } else {
                                "HTTP/1.1 200 OK\r\n\
                                content-length: 0\r\n\
                            \r\n"
                            };
                            socket.write_all(response.as_bytes()).await.unwrap();
                        });
                    }
                }
            }
        });
        (addr_str, shutdown_tx)
    }

    #[tokio::test]
    async fn test_serve_sink() {
        let serve_sink = ServeSink::new();

        let (addr, shutdown_tx) = start_server().await;

        let (tx, rx) = mpsc::channel(1);

        let mut headers = HashMap::new();
        headers.insert(NUMAFLOW_ID_HEADER.to_string(), "12345".to_string());

        headers.insert(
            NUMAFLOW_CALLBACK_URL_HEADER.to_string(),
            format!("http://{}/sync", addr),
        );

        let request = SinkRequest {
            keys: vec![],
            id: "1".to_string(),
            value: b"test".to_vec(),
            watermark: Default::default(),
            headers,
            event_time: Default::default(),
        };

        tx.send(request).await.unwrap();
        drop(tx); // Close the sender to end the stream

        let responses = serve_sink.sink(rx).await;
        assert_eq!(responses.len(), 1);
        assert!(responses[0].success);

        // Stop the server
        shutdown_tx.send(()).await.unwrap();
    }
}
