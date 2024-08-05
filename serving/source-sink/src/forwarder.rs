use tokio::sync::oneshot;
use tokio::task::JoinSet;
use tracing::info;

use crate::error::{Error, Result};
use crate::sink::SinkClient;
use crate::source::SourceClient;
use crate::transformer::TransformerClient;

pub(crate) struct Forwarder {
    source_client: SourceClient,
    sink_client: SinkClient,
    transformer_client: Option<TransformerClient>,
    timeout_in_ms: u32,
    batch_size: u64,
    shutdown_rx: oneshot::Receiver<()>,
}

impl Forwarder {
    pub(crate) async fn new(
        source_client: SourceClient,
        sink_client: SinkClient,
        transformer_client: Option<TransformerClient>,
        timeout_in_ms: u32,
        batch_size: u64,
        shutdown_rx: oneshot::Receiver<()>,
    ) -> Result<Self> {
        Ok(Self {
            source_client,
            sink_client,
            transformer_client,
            timeout_in_ms,
            batch_size,
            shutdown_rx,
        })
    }

    pub(crate) async fn run(&mut self) -> Result<()> {
        loop {
            tokio::select! {
                _ = &mut self.shutdown_rx => {
                    info!("Shutdown signal received, stopping forwarder...");
                    break;
                }
                result = self.source_client.read_fn(self.batch_size, self.timeout_in_ms) => {
                    // Read messages from the source
                    let messages = result?;

                    // Extract offsets from the messages
                    let offsets = messages.iter().map(|message| message.offset.clone()).collect();

                    // Apply transformation if transformer is present
                    let transformed_messages = if let Some(transformer_client) = &self.transformer_client {
                        let mut jh = JoinSet::new();
                        for message in messages {
                            let mut transformer_client = transformer_client.clone();
                            jh.spawn(async move { transformer_client.transform_fn(message).await });
                        }

                        let mut results = Vec::new();
                        while let Some(task) = jh.join_next().await {
                            let result = task.map_err(|e| Error::TransformerError(format!("{:?}", e)))?;
                            let result = result?;
                            results.extend(result);
                        }
                        results
                    } else {
                        messages
                    };

                    // Write messages to the sink
                    self.sink_client.sink_fn(transformed_messages).await?;

                    // Acknowledge the messages
                    self.source_client.ack_fn(offsets).await?;
                }
            }
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashSet;

    use chrono::Utc;
    use numaflow::source::{Message, Offset, SourceReadRequest};
    use numaflow::{sink, source, sourcetransform};
    use tokio::sync::mpsc::Sender;

    use crate::forwarder::Forwarder;
    use crate::sink::SinkClient;
    use crate::source::SourceClient;
    use crate::transformer::TransformerClient;

    struct SimpleSource {
        yet_to_be_acked: std::sync::RwLock<HashSet<String>>,
    }

    impl SimpleSource {
        fn new() -> Self {
            Self {
                yet_to_be_acked: std::sync::RwLock::new(HashSet::new()),
            }
        }
    }

    #[tonic::async_trait]
    impl source::Sourcer for SimpleSource {
        async fn read(&self, request: SourceReadRequest, transmitter: Sender<Message>) {
            let event_time = Utc::now();
            let mut message_offsets = Vec::with_capacity(request.count);
            for i in 0..2 {
                let offset = format!("{}-{}", event_time.timestamp_nanos_opt().unwrap(), i);
                transmitter
                    .send(Message {
                        value: "test-message".as_bytes().to_vec(),
                        event_time,
                        offset: Offset {
                            offset: offset.clone().into_bytes(),
                            partition_id: 0,
                        },
                        keys: vec!["test-key".to_string()],
                        headers: Default::default(),
                    })
                    .await
                    .unwrap();
                message_offsets.push(offset)
            }
            self.yet_to_be_acked
                .write()
                .unwrap()
                .extend(message_offsets)
        }

        async fn ack(&self, offsets: Vec<Offset>) {
            for offset in offsets {
                self.yet_to_be_acked
                    .write()
                    .unwrap()
                    .remove(&String::from_utf8(offset.offset).unwrap());
            }
        }

        async fn pending(&self) -> usize {
            self.yet_to_be_acked.read().unwrap().len()
        }

        async fn partitions(&self) -> Option<Vec<i32>> {
            Some(vec![0])
        }
    }

    struct SimpleTransformer;
    #[tonic::async_trait]
    impl sourcetransform::SourceTransformer for SimpleTransformer {
        async fn transform(
            &self,
            input: sourcetransform::SourceTransformRequest,
        ) -> Vec<sourcetransform::Message> {
            let keys = input
                .keys
                .iter()
                .map(|k| k.clone() + "-transformed")
                .collect();
            let message = sourcetransform::Message::new(input.value, Utc::now())
                .keys(keys)
                .tags(vec![]);
            vec![message]
        }
    }

    struct InMemorySink {
        sender: Sender<Message>,
    }

    impl InMemorySink {
        fn new(sender: Sender<Message>) -> Self {
            Self { sender }
        }
    }

    #[tonic::async_trait]
    impl sink::Sinker for InMemorySink {
        async fn sink(
            &self,
            mut input: tokio::sync::mpsc::Receiver<sink::SinkRequest>,
        ) -> Vec<sink::Response> {
            let mut responses: Vec<sink::Response> = Vec::new();
            while let Some(datum) = input.recv().await {
                let response = match std::str::from_utf8(&datum.value) {
                    Ok(_) => {
                        self.sender
                            .send(Message {
                                value: datum.value.clone(),
                                event_time: datum.event_time,
                                offset: Offset {
                                    offset: "test-offset".to_string().into_bytes(),
                                    partition_id: 0,
                                },
                                keys: datum.keys.clone(),
                                headers: Default::default(),
                            })
                            .await
                            .unwrap();
                        sink::Response::ok(datum.id)
                    }
                    Err(e) => {
                        sink::Response::failure(datum.id, format!("Invalid UTF-8 sequence: {}", e))
                    }
                };
                responses.push(response);
            }
            responses
        }
    }

    #[tokio::test]
    async fn test_forwarder_source_sink() {
        // Create channels for communication
        let (sink_tx, mut sink_rx) = tokio::sync::mpsc::channel(10);

        // Start the source server
        let (source_shutdown_tx, source_shutdown_rx) = tokio::sync::oneshot::channel();
        let tmp_dir = tempfile::TempDir::new().unwrap();
        let source_sock_file = tmp_dir.path().join("source.sock");

        let source_socket = source_sock_file.clone();
        let source_server_handle = tokio::spawn(async move {
            let server_info_file = tmp_dir.path().join("source-server-info");
            source::Server::new(SimpleSource::new())
                .with_socket_file(source_socket)
                .with_server_info_file(server_info_file)
                .start_with_shutdown(source_shutdown_rx)
                .await
                .unwrap();
        });

        // Start the sink server
        let (sink_shutdown_tx, sink_shutdown_rx) = tokio::sync::oneshot::channel();
        let sink_tmp_dir = tempfile::TempDir::new().unwrap();
        let sink_sock_file = sink_tmp_dir.path().join("sink.sock");

        let sink_socket = sink_sock_file.clone();
        let sink_server_handle = tokio::spawn(async move {
            let server_info_file = sink_tmp_dir.path().join("sink-server-info");
            sink::Server::new(InMemorySink::new(sink_tx))
                .with_socket_file(sink_socket)
                .with_server_info_file(server_info_file)
                .start_with_shutdown(sink_shutdown_rx)
                .await
                .unwrap();
        });

        // Start the transformer server
        let (transformer_shutdown_tx, transformer_shutdown_rx) = tokio::sync::oneshot::channel();
        let tmp_dir = tempfile::TempDir::new().unwrap();
        let transformer_sock_file = tmp_dir.path().join("transformer.sock");

        let transformer_socket = transformer_sock_file.clone();
        let transformer_server_handle = tokio::spawn(async move {
            let server_info_file = tmp_dir.path().join("transformer-server-info");
            sourcetransform::Server::new(SimpleTransformer)
                .with_socket_file(transformer_socket)
                .with_server_info_file(server_info_file)
                .start_with_shutdown(transformer_shutdown_rx)
                .await
                .unwrap();
        });

        // Wait for the servers to start
        tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;

        let (forwarder_shutdown_tx, forwarder_shutdown_rx) = tokio::sync::oneshot::channel();

        let source_client = SourceClient::connect(source_sock_file)
            .await
            .expect("failed to connect to source server");

        let sink_client = SinkClient::connect(sink_sock_file)
            .await
            .expect("failed to connect to sink server");

        let transformer_client = TransformerClient::connect(transformer_sock_file)
            .await
            .expect("failed to connect to transformer server");

        let mut forwarder = Forwarder::new(
            source_client,
            sink_client,
            Some(transformer_client),
            1000,
            10,
            forwarder_shutdown_rx,
        )
        .await
        .expect("failed to create forwarder");

        let forwarder_handle = tokio::spawn(async move {
            forwarder.run().await.unwrap();
        });

        // Receive messages from the sink
        let received_message = sink_rx.recv().await.unwrap();
        assert_eq!(received_message.value, "test-message".as_bytes());
        assert_eq!(
            received_message.keys,
            vec!["test-key-transformed".to_string()]
        );

        // stop the forwarder
        forwarder_shutdown_tx
            .send(())
            .expect("failed to send shutdown signal");
        forwarder_handle
            .await
            .expect("failed to join forwarder task");

        // stop the servers
        source_shutdown_tx
            .send(())
            .expect("failed to send shutdown signal");
        source_server_handle
            .await
            .expect("failed to join source server task");

        transformer_shutdown_tx
            .send(())
            .expect("failed to send shutdown signal");
        transformer_server_handle
            .await
            .expect("failed to join transformer server task");

        sink_shutdown_tx
            .send(())
            .expect("failed to send shutdown signal");
        sink_server_handle
            .await
            .expect("failed to join sink server task");
    }
}
