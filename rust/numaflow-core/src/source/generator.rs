use futures::StreamExt;

use crate::config::components::source::GeneratorConfig;
use crate::message::{Message, Offset};
use crate::reader;
use crate::source;

/// Stream Generator returns a set of messages for every `.next` call. It will throttle itself if
/// the call exceeds the RPU. It will return a max (batch size, RPU) till the quota for that unit of
/// time is over. If `.next` is called after the quota is over, it will park itself so that it won't
/// return more than the RPU. Once parked, it will unpark itself and return as soon as the next poll
/// happens.
/// We skip the missed ticks because there is no point to give a burst, most likely that burst cannot
/// be absorbed.
/// ```text
///       Ticks: |     1     |     2     |     3     |     4     |     5     |     6     |
///              =========================================================================> time
///  Read RPU=5: | :xxx:xx:  | :xxx <delay>             |:xxx:xx:| :xxx:xx:  | :xxx:xx:  |
///                2 batches   only 1 batch (no reread)      5         5           5
///                 
/// ```
/// NOTE: The minimum granularity of duration is 10ms.
mod stream_generator {
    use std::pin::Pin;
    use std::task::{Context, Poll};
    use std::time::Duration;

    use bytes::Bytes;
    use futures::Stream;
    use pin_project::pin_project;
    use rand::Rng;
    use tokio::time::MissedTickBehavior;
    use tracing::warn;

    use crate::config::components::source::GeneratorConfig;
    use crate::message::{
        get_vertex_name, get_vertex_replica, Message, MessageID, Offset, StringOffset,
    };

    #[pin_project]
    pub(super) struct StreamGenerator {
        /// the content generated by Generator.
        content: Bytes,
        /// requests per unit of time-period.
        rpu: usize,
        /// batch size per read
        batch: usize,
        /// the amount of credits used for the current time-period.
        /// remaining = (rpu - used) for that time-period
        used: usize,
        /// const int data to be send in the payload if provided by the user.
        /// If `content` is present, this will be ignored.
        /// This is a simple way used by users to test Reduce feature.
        value: Option<i64>,
        /// total message size to be created, will be padded with random u8. Size is
        /// only an approximation.
        msg_size_bytes: u32,
        /// Vary the event-time of the messages to produce some out-of-orderliness. It is in
        /// seconds granularity.
        jitter: Duration,
        /// keys to be used for the messages and the current index in the list
        /// All possible keys are generated in the constructor.
        /// The index is incremented (treating key list as cyclic) when a message is generated.
        keys: (Vec<String>, usize),
        #[pin]
        tick: tokio::time::Interval,
    }

    impl StreamGenerator {
        pub(super) fn new(cfg: GeneratorConfig, batch_size: usize) -> Self {
            let mut tick = tokio::time::interval(cfg.duration);
            tick.set_missed_tick_behavior(MissedTickBehavior::Skip);

            let mut rpu = cfg.rpu;
            // Key count cannot be more than RPU.
            // If rpu is not a multiple of the key_count, we floor the rpu to the nearest multiple of key_count
            // We cap the key_count to u8::MAX in config.rs
            let key_count = std::cmp::min(cfg.key_count as usize, cfg.rpu) as u8;
            if key_count != cfg.key_count {
                warn!(
                    "Specified KeyCount({}) is higher than RPU ({}). KeyCount is changed to {}",
                    cfg.key_count, cfg.rpu, key_count
                );
            }
            if key_count > 0 && rpu % key_count as usize != 0 {
                let new_rpu = rpu - (rpu % key_count as usize);
                warn!(rpu, key_count, "Specified RPU is not a multiple of the KeyCount. This may lead to uneven distribution of messages across keys. RPUs will be adjusted to {}", new_rpu);
                rpu = new_rpu;
            }

            // Generate all possible keys
            let keys = (0..key_count).map(|i| format!("key-{}", i)).collect();

            Self {
                content: cfg.content,
                rpu,
                // batch cannot > rpu
                batch: std::cmp::min(cfg.rpu, batch_size),
                used: 0,
                tick,
                value: cfg.value,
                msg_size_bytes: cfg.msg_size_bytes,
                keys: (keys, 0),
                jitter: cfg.jitter,
            }
        }

        /// Generates a similar payload as the Go implementation.
        /// This is only needed if the user has not specified `valueBlob` in the generator source configuration in the pipeline
        fn generate_payload(&self, value: i64) -> Vec<u8> {
            #[derive(serde::Serialize)]
            struct Data {
                value: i64,
                // only to ensure a desired message size
                #[serde(skip_serializing_if = "Vec::is_empty")]
                padding: Vec<u8>,
            }

            let padding: Vec<u8> = (self.msg_size_bytes > 8)
                .then(|| {
                    let size = self.msg_size_bytes - 8;
                    let mut bytes = vec![0; size as usize];
                    rand::thread_rng().fill(&mut bytes[..]);
                    bytes
                })
                .unwrap_or_default();

            let data = Data { value, padding };
            serde_json::to_vec(&data).unwrap()
        }

        /// we have a global array of prepopulated keys, we just have to fetch the next in line.
        /// to fetch the next one, we idx++ whenever we fetch.
        /// This will be a single element vector at the most.
        fn next_key_to_be_fetched(&mut self) -> Vec<String> {
            let idx = self.keys.1;
            // fetches the next key from the predefined set of keys.
            match self.keys.0.get(idx) {
                Some(key) => {
                    self.keys.1 = (idx + 1) % self.keys.0.len();
                    vec![key.clone()]
                }
                None => vec![],
            }
        }

        /// creates a single message that can be returned by the generator.
        fn create_message(&mut self) -> Message {
            let id = chrono::Utc::now()
                .timestamp_nanos_opt()
                .unwrap_or_default()
                .to_string();

            let offset = Offset::String(StringOffset::new(id.clone(), *get_vertex_replica()));

            // rng.gen_range(0..0) panics with "cannot sample empty range"
            // rng.gen_range(0..1) will always produce 0
            let jitter = self.jitter.as_secs().max(1);
            let event_time =
                chrono::Utc::now() - Duration::from_secs(rand::thread_rng().gen_range(0..jitter));
            let mut data = self.content.to_vec();
            if data.is_empty() {
                let value = match self.value {
                    Some(v) => v,
                    None => event_time.timestamp_nanos_opt().unwrap_or_default(),
                };
                data = self.generate_payload(value);
            }

            Message {
                keys: self.next_key_to_be_fetched(),
                value: data.into(),
                offset: Some(offset.clone()),
                event_time,
                id: MessageID {
                    vertex_name: get_vertex_name().to_string(),
                    offset: offset.to_string(),
                    index: Default::default(),
                },
                headers: Default::default(),
            }
        }

        /// generates a set of messages to be returned.
        fn generate_messages(&mut self, count: usize) -> Vec<Message> {
            let mut data = Vec::with_capacity(count);
            for _ in 0..count {
                data.push(self.create_message());
            }
            data
        }
    }

    impl Stream for StreamGenerator {
        type Item = Vec<Message>;

        fn poll_next(
            mut self: Pin<&mut StreamGenerator>,
            cx: &mut Context<'_>,
        ) -> Poll<Option<Self::Item>> {
            let mut this = self.as_mut().project();
            match this.tick.poll_tick(cx) {
                // Poll::Ready means we are ready to send data the whole batch since enough time
                // has passed.
                Poll::Ready(_) => {
                    *this.used = *this.batch;
                    let count = self.batch;
                    let data = self.generate_messages(count);
                    // reset used quota
                    Poll::Ready(Some(data))
                }
                Poll::Pending => {
                    // even if enough time hasn't passed, we can still send data if we have
                    // quota (rpu - used) left
                    if this.used < this.rpu {
                        // make sure we do not send more than desired
                        let to_send = std::cmp::min(*this.rpu - *this.used, *this.batch);

                        // update the counters
                        *this.used += to_send;
                        let data = self.generate_messages(to_send);
                        Poll::Ready(Some(data))
                    } else {
                        Poll::Pending
                    }
                }
            }
        }

        /// size is roughly what is remaining and upper bound is for sure RPU. This is a very
        /// rough approximation because Duration is not taken into account for the lower bound.
        fn size_hint(&self) -> (usize, Option<usize>) {
            (self.rpu - self.used, Some(self.rpu))
        }
    }

    #[cfg(test)]
    mod tests {
        use futures::StreamExt;

        use super::*;

        #[tokio::test]
        async fn test_stream_generator() {
            // Define the content to be generated
            let content = Bytes::from("test_data");
            // Define requests per unit (rpu), batch size, and time unit
            let batch = 6;
            let rpu = 10;
            let cfg = GeneratorConfig {
                content: content.clone(),
                rpu,
                jitter: Duration::from_millis(0),
                duration: Duration::from_millis(100),
                ..Default::default()
            };

            // Create a new StreamGenerator
            let mut stream_generator = StreamGenerator::new(cfg, batch);

            // Collect the first batch of data
            let first_batch = stream_generator.next().await.unwrap();
            assert_eq!(first_batch.len(), batch);
            for item in first_batch {
                assert_eq!(item.value, content);
            }

            // Collect the second batch of data
            let second_batch = stream_generator.next().await.unwrap();
            assert_eq!(second_batch.len(), rpu - batch);
            for item in second_batch {
                assert_eq!(item.value, content);
            }

            // no there is no more data left in the quota
            let size = stream_generator.size_hint();
            assert_eq!(size.0, 0);
            assert_eq!(size.1, Some(rpu));

            let third_batch = stream_generator.next().await.unwrap();
            assert_eq!(third_batch.len(), 6);
            for item in third_batch {
                assert_eq!(item.value, content);
            }

            // we should now have data
            let size = stream_generator.size_hint();
            assert_eq!(size.0, 4);
            assert_eq!(size.1, Some(rpu));
        }

        #[tokio::test]
        async fn test_stream_generator_config() {
            let cfg = GeneratorConfig {
                rpu: 33,
                key_count: 7,
                ..Default::default()
            };

            let stream_generator = StreamGenerator::new(cfg, 50);
            assert_eq!(stream_generator.rpu, 28);

            let cfg = GeneratorConfig {
                rpu: 3,
                key_count: 7,
                ..Default::default()
            };
            let stream_generator = StreamGenerator::new(cfg, 30);
            assert_eq!(stream_generator.keys.0.len(), 3);
        }
    }
}

/// Creates a new generator and returns all the necessary implementation of the Source trait.
/// Generator Source is mainly used for development purpose, where you want to have self-contained
/// source to generate some messages. We mainly use generator for load testing and integration
/// testing of Numaflow. The load generated is per replica.
pub(crate) fn new_generator(
    cfg: GeneratorConfig,
    batch_size: usize,
) -> crate::Result<(GeneratorRead, GeneratorAck, GeneratorLagReader)> {
    let gen_read = GeneratorRead::new(cfg, batch_size);
    let gen_ack = GeneratorAck::new();
    let gen_lag_reader = GeneratorLagReader::new();

    Ok((gen_read, gen_ack, gen_lag_reader))
}

pub(crate) struct GeneratorRead {
    stream_generator: stream_generator::StreamGenerator,
}

impl GeneratorRead {
    /// A new [GeneratorRead] is returned. It takes a static content, requests per unit-time, batch size
    /// to return per [source::SourceReader::read], and the unit-time as duration.
    fn new(cfg: GeneratorConfig, batch_size: usize) -> Self {
        let stream_generator = stream_generator::StreamGenerator::new(cfg.clone(), batch_size);
        Self { stream_generator }
    }
}

impl source::SourceReader for GeneratorRead {
    fn name(&self) -> &'static str {
        "generator"
    }

    async fn read(&mut self) -> crate::error::Result<Vec<Message>> {
        let Some(messages) = self.stream_generator.next().await else {
            panic!("Stream generator has stopped");
        };
        Ok(messages)
    }

    fn partitions(&self) -> Vec<u16> {
        todo!()
    }
}

pub(crate) struct GeneratorAck {}

impl GeneratorAck {
    fn new() -> Self {
        Self {}
    }
}

impl source::SourceAcker for GeneratorAck {
    async fn ack(&mut self, _: Vec<Offset>) -> crate::error::Result<()> {
        Ok(())
    }
}

#[derive(Clone)]
pub(crate) struct GeneratorLagReader {}

impl GeneratorLagReader {
    fn new() -> Self {
        Self {}
    }
}

impl reader::LagReader for GeneratorLagReader {
    async fn pending(&mut self) -> crate::error::Result<Option<usize>> {
        // Generator is not meant to auto-scale.
        Ok(None)
    }
}

#[cfg(test)]
mod tests {
    use bytes::Bytes;
    use tokio::time::Duration;

    use super::*;
    use crate::message::StringOffset;
    use crate::reader::LagReader;
    use crate::source::{SourceAcker, SourceReader};

    #[tokio::test]
    async fn test_generator_read() {
        // Define the content to be generated
        let content = Bytes::from("test_data");
        // Define requests per unit (rpu), batch size, and time unit
        let rpu = 10;
        let batch = 5;
        let cfg = GeneratorConfig {
            content: content.clone(),
            rpu,
            jitter: Duration::from_millis(0),
            duration: Duration::from_millis(100),
            ..Default::default()
        };

        // Create a new Generator
        let mut generator = GeneratorRead::new(cfg, batch);

        // Read the first batch of messages
        let messages = generator.read().await.unwrap();
        assert_eq!(messages.len(), batch);

        assert!(messages.first().unwrap().value.eq(&content));

        // Verify that each message has the expected structure

        // Read the second batch of messages
        let messages = generator.read().await.unwrap();
        assert_eq!(messages.len(), rpu - batch);
    }

    #[tokio::test]
    async fn test_generator_read_with_random_data() {
        // Here we do not provide any content, so the generator will generate random data
        // Define requests per unit (rpu), batch size, and time unit
        let rpu = 10;
        let batch = 5;
        let cfg = GeneratorConfig {
            content: Bytes::new(),
            rpu,
            jitter: Duration::from_millis(0),
            duration: Duration::from_millis(100),
            key_count: 3,
            msg_size_bytes: 100,
            ..Default::default()
        };

        // Create a new Generator
        let mut generator = GeneratorRead::new(cfg, batch);

        // Read the first batch of messages
        let messages = generator.read().await.unwrap();
        let keys = messages
            .iter()
            .map(|m| m.keys[0].clone())
            .collect::<Vec<_>>();

        let expected_keys = vec![
            "key-0".to_string(),
            "key-1".to_string(),
            "key-2".to_string(),
            "key-0".to_string(),
            "key-1".to_string(),
        ];

        assert_eq!(keys, expected_keys);
        assert!(messages.first().unwrap().value.len() >= 100);

        assert_eq!(messages.len(), batch);
    }

    #[tokio::test]
    async fn test_generator_lag_pending() {
        // Create a new GeneratorLagReader
        let mut lag_reader = GeneratorLagReader::new();

        // Call the pending method and check the result
        let pending_result = lag_reader.pending().await;

        // Assert that the result is Ok(None)
        assert!(pending_result.is_ok());
        assert_eq!(pending_result.unwrap(), None);
    }

    #[tokio::test]
    async fn test_generator_ack() {
        // Create a new GeneratorAck instance
        let mut generator_ack = GeneratorAck::new();

        // Create a vector of offsets to acknowledge
        let offsets = vec![
            Offset::String(StringOffset::new("offset1".to_string(), 0)),
            Offset::String(StringOffset::new("offset2".to_string(), 0)),
        ];

        // Call the ack method and check the result
        let ack_result = generator_ack.ack(offsets).await;

        // Assert that the result is Ok(())
        assert!(ack_result.is_ok());
    }
}
