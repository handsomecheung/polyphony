use std::collections::HashSet;
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};

#[derive(Clone, Debug)]
pub struct StreamMasker {
    secrets: Vec<String>,
}

impl StreamMasker {
    pub fn new<I, S>(secrets: I) -> Self
    where
        I: IntoIterator<Item = S>,
        S: AsRef<str>,
    {
        let mut unique: HashSet<String> = HashSet::new();
        for s in secrets {
            let str_ref = s.as_ref();
            // Per SPEC: redact occurrences of secrets with length >= 4 bytes
            if str_ref.len() >= 4 {
                unique.insert(str_ref.to_string());
            }
        }

        // Sort descending by length so longer substrings are masked first
        let mut sorted: Vec<String> = unique.into_iter().collect();
        sorted.sort_by_key(|b| std::cmp::Reverse(b.len()));

        Self { secrets: sorted }
    }

    /// Redacts all known secrets from the given text string.
    pub fn mask(&self, text: &str) -> String {
        if self.secrets.is_empty() {
            return text.to_string();
        }

        let mut result = text.to_string();
        for secret in &self.secrets {
            result = result.replace(secret, "[TACET_MASKED]");
        }
        result
    }

    /// Streams and redacts lines from an AsyncRead source to an AsyncWrite destination in real time.
    pub async fn pipe_masked<R, W>(&self, reader: R, mut writer: W) -> std::io::Result<()>
    where
        R: tokio::io::AsyncRead + Unpin,
        W: tokio::io::AsyncWrite + Unpin,
    {
        let mut lines = BufReader::new(reader).lines();
        while let Some(line) = lines.next_line().await? {
            let masked = self.mask(&line);
            writer.write_all(masked.as_bytes()).await?;
            writer.write_all(b"\n").await?;
            writer.flush().await?;
        }
        Ok(())
    }
}
