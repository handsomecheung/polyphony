use serde::{Deserialize, Serialize};
use std::fs;
use std::net::SocketAddr;
use std::time::{Duration, Instant};
use anyhow::{Result, anyhow};
use reqwest::Client;
use reqwest::redirect::Policy;
use hickory_resolver::TokioAsyncResolver;
use hickory_resolver::config::{ResolverConfig, ResolverOpts, NameServerConfig, Protocol};
use tokio::net::TcpStream;
use tokio::sync::Semaphore;
use jsonpath_lib::select;
use futures::future::join_all;
use std::sync::Arc;
use std::env;
use regex::Regex;
use google_cloud_storage::client::Storage;



#[derive(Serialize)]
struct TaskResult {
    name: String,
    #[serde(rename = "type")]
    task_type: String,
    status: String,
    latency_ms: u128,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

#[derive(Debug, Deserialize)]
struct Config {
    globals: Option<Globals>,
    tasks: Option<Vec<Task>>,
}

#[derive(Debug, Deserialize, Clone)]
struct RecentSummaryConfig {
    entrypoint: String,
    count: usize,
    directory: Option<String>,
    filename: String,
}

#[derive(Debug, Deserialize, Clone)]
struct UploadConfig {
    #[serde(rename = "type")]
    upload_type: String,
    bucket: String,
    directory: Option<String>,
    credential: Option<String>,
    skip_empty: Option<bool>,
    recent_summary: Option<RecentSummaryConfig>,
}

#[derive(Debug, Deserialize)]
struct Globals {
    timeout: Option<String>,
    upload: Option<UploadConfig>,
}

#[derive(Debug, Deserialize, Clone)]
struct Task {
    name: String,
    #[serde(rename = "type")]
    task_type: Option<String>,
    target: Option<String>,
    host: Option<String>,
    port: Option<u16>,
    method: Option<String>,
    headers: Option<std::collections::HashMap<String, String>>,
    body: Option<String>,
    expect: Option<Expect>,
    #[allow(dead_code)]
    sla: Option<Sla>,
    #[allow(dead_code)]
    count: Option<u32>,
    expected_records: Option<Vec<String>>,
    unexpected_records: Option<Vec<String>>,
    server: Option<String>,
    #[allow(dead_code)]
    alert_days_before: Option<i64>,
    timeout: Option<String>,
    range: Option<String>,
    ports: Option<Vec<u16>>,
    expect_open: Option<Vec<u16>>,
    expect_closed: Option<Vec<u16>>,
    disabled: Option<bool>,
}

impl Task {
    fn merge(&mut self, other: Task) {
        if other.task_type.is_some() {
            self.task_type = other.task_type;
        }
        if other.target.is_some() {
            self.target = other.target;
        }
        if other.host.is_some() {
            self.host = other.host;
        }
        if other.port.is_some() {
            self.port = other.port;
        }
        if other.method.is_some() {
            self.method = other.method;
        }
        if other.headers.is_some() {
            self.headers = other.headers;
        }
        if other.body.is_some() {
            self.body = other.body;
        }
        if other.expect.is_some() {
            self.expect = other.expect;
        }
        if other.sla.is_some() {
            self.sla = other.sla;
        }
        if other.count.is_some() {
            self.count = other.count;
        }
        if other.expected_records.is_some() {
            self.expected_records = other.expected_records;
        }
        if other.unexpected_records.is_some() {
            self.unexpected_records = other.unexpected_records;
        }
        if other.server.is_some() {
            self.server = other.server;
        }
        if other.alert_days_before.is_some() {
            self.alert_days_before = other.alert_days_before;
        }
        if other.timeout.is_some() {
            self.timeout = other.timeout;
        }
        if other.range.is_some() {
            self.range = other.range;
        }
        if other.ports.is_some() {
            self.ports = other.ports;
        }
        if other.expect_open.is_some() {
            self.expect_open = other.expect_open;
        }
        if other.expect_closed.is_some() {
            self.expect_closed = other.expect_closed;
        }
        if other.disabled.is_some() {
            self.disabled = other.disabled;
        }
    }
}

#[derive(Debug, Deserialize, Clone)]
struct Expect {
    status: Option<u16>,
    body: Option<Vec<String>>,
    json: Option<std::collections::HashMap<String, String>>,
}

#[derive(Debug, Deserialize, Clone)]
struct Sla {
    #[allow(dead_code)]
    latency: String,
}

fn parse_duration(d: &str) -> Duration {
    let s = d.replace("s", "").replace("ms", "");
    let val: u64 = s.parse().unwrap_or(3);
    if d.contains("ms") {
        Duration::from_millis(val)
    } else {
        Duration::from_secs(val)
    }
}

async fn get_resolver(task_server: &Option<String>) -> Result<TokioAsyncResolver> {
    if let Some(server_addr) = task_server {
        let ip = server_addr.parse::<std::net::IpAddr>()
            .map_err(|_| anyhow!("Invalid DNS server IP: {}", server_addr))?;
        let config = ResolverConfig::from_parts(
            None,
            vec![],
            vec![NameServerConfig::new(SocketAddr::new(ip, 53), Protocol::Udp)],
        );
        Ok(TokioAsyncResolver::tokio(config, ResolverOpts::default()))
    } else {
        Ok(TokioAsyncResolver::tokio_from_system_conf()?)
    }
}

fn collect_files_recursive(path: &std::path::Path, files: &mut Vec<std::path::PathBuf>) -> Result<()> {
    if path.is_dir() {
        for entry in fs::read_dir(path)? {
            let entry = entry?;
            let p = entry.path();
            if p.is_dir() {
                collect_files_recursive(&p, files)?;
            } else if p.is_file() {
                if let Some(ext) = p.extension() {
                    if ext == "yaml" || ext == "yml" {
                        files.push(p);
                    }
                }
            }
        }
    }
    Ok(())
}

#[tokio::main]
async fn main() -> Result<()> {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let args: Vec<String> = env::args().collect();
    let mut config_files = Vec::new();
    let mut concurrency = 64;
    let mut only_failures = false;
    let mut format = "json".to_string();
    
    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "--config-file" if i + 1 < args.len() => {
                config_files.push(args[i + 1].clone());
                i += 2;
            }
            "--config-dir" if i + 1 < args.len() => {
                let dir = &args[i + 1];
                let mut dir_files = Vec::new();
                collect_files_recursive(std::path::Path::new(dir), &mut dir_files)
                    .map_err(|e| anyhow!("Failed to read config directory {}: {}", dir, e))?;
                dir_files.sort(); // Consistent order
                for path in dir_files {
                    config_files.push(path.to_string_lossy().into_owned());
                }
                i += 2;
            }
            "--concurrency" | "-c" if i + 1 < args.len() => {
                concurrency = args[i + 1].parse::<usize>()
                    .map_err(|_| anyhow!("Invalid concurrency value: {}", args[i + 1]))?;
                i += 2;
            }
            "--only-failures" => {
                only_failures = true;
                i += 1;
            }
            "--format" if i + 1 < args.len() => {
                let fmt = args[i + 1].to_lowercase();
                if fmt == "json" || fmt == "plain" {
                    format = fmt;
                } else {
                    return Err(anyhow!("Invalid format value: {}. Allowed values: json, plain", args[i + 1]));
                }
                i += 2;
            }
            _ => i += 1,
        }
    }

    if config_files.is_empty() {
        eprintln!("Usage: {} --config-file <file.yaml> | --config-dir <dir> [--concurrency <n>] [--only-failures] [--format <json|plain>]", args[0]);
        std::process::exit(1);
    }

    let mut all_tasks: Vec<Task> = Vec::new();
    let mut merged_timeout = None;
    let mut merged_upload = None;

    for path in config_files {
        let data = fs::read_to_string(&path)
            .map_err(|e| anyhow!("Failed to read config file {}: {}", path, e))?;
        let config: Config = serde_yaml::from_str(&data)
            .map_err(|e| anyhow!("Failed to parse config file {}: {}", path, e))?;
        
        if let Some(globals) = config.globals {
            if let Some(timeout) = globals.timeout {
                merged_timeout = Some(timeout);
            }
            if let Some(upload) = globals.upload {
                merged_upload = Some(upload);
            }
        }
        
        if let Some(tasks) = config.tasks {
            for new_task in tasks {
                if let Some(existing_task) = all_tasks.iter_mut().find(|t| t.name == new_task.name) {
                    existing_task.merge(new_task);
                } else {
                    all_tasks.push(new_task);
                }
            }
        }
    }

    let active_tasks: Vec<Task> = all_tasks.into_iter()
        .filter(|t| !t.disabled.unwrap_or(false))
        .collect();

    for task in &active_tasks {
        if task.task_type.is_none() {
            return Err(anyhow!("Task '{}' is missing required field 'type'", task.name));
        }
    }

    let timeout_str = merged_timeout.unwrap_or_else(|| "5s".to_string());
    let global_timeout = parse_duration(&timeout_str);

    eprintln!("Starting execution of {} tasks...", active_tasks.len());
    let start_time = Instant::now();

    let semaphore = Arc::new(Semaphore::new(concurrency));
    let mut handles = Vec::new();

    for task in active_tasks {
        let sem = semaphore.clone();
        let handle = tokio::spawn(async move {
            let start = Instant::now();
            let task_type_str = task.task_type.as_deref().unwrap_or("");
            let result = match task_type_str {
                "http" => check_http(&task, global_timeout, sem).await,
                "tcp" => check_tcp(&task, global_timeout, sem).await,
                "dns" => check_dns(&task, sem).await,
                "ssl" => check_ssl(&task, global_timeout, sem).await,
                "port_scan" => check_port_scan(&task, sem).await,
                "noindex" => check_noindex(&task, global_timeout, sem).await,
                _ => Err(anyhow!("Unknown task type: {}", task_type_str)),
            };
            let duration = start.elapsed();

            let (status, error) = match result {
                Ok(_) => ("OK".to_string(), None),
                Err(e) => ("FAIL".to_string(), Some(e.to_string())),
            };

            TaskResult {
                name: task.name,
                task_type: task.task_type.unwrap_or_default(),
                status,
                latency_ms: duration.as_millis(),
                error,
            }
        });
        handles.push(handle);
    }

    let results = join_all(handles).await;
    let elapsed = start_time.elapsed();
    let mut task_results: Vec<TaskResult> = results.into_iter()
        .filter_map(|r| r.ok())
        .collect();

    let total = task_results.len();
    let succeeded = task_results.iter().filter(|r| r.status == "OK").count();
    let failed = task_results.iter().filter(|r| r.status == "FAIL").count();

    if only_failures {
        task_results.retain(|r| r.status == "FAIL");
    }

    let (output_data, ext) = if format == "plain" {
        let mut plain_text = String::new();
        for r in &task_results {
            if let Some(err) = &r.error {
                plain_text.push_str(&format!("[{}] {} ({}): {} ({}ms)\n", r.status, r.name, r.task_type, err, r.latency_ms));
            } else {
                plain_text.push_str(&format!("[{}] {} ({}): OK ({}ms)\n", r.status, r.name, r.task_type, r.latency_ms));
            }
        }
        (plain_text, "txt")
    } else {
        (serde_json::to_string_pretty(&task_results)?, "json")
    };

    if format == "plain" {
        print!("{}", output_data);
    } else {
        println!("{}", output_data);
    }

    if let Some(upload) = merged_upload {
        if upload.upload_type == "gcs" {
            let skip_empty = upload.skip_empty.unwrap_or(true);
            let is_empty = task_results.is_empty();

            if skip_empty && is_empty {
                eprintln!("Test results are empty. Skipping GCS upload as configured.");
            } else {
                let timestamp = chrono::Utc::now().format("%Y%m%d_%H%M%S").to_string();
                let object_name = if let Some(dir) = &upload.directory {
                    format!("{}/results_{}.{}", dir.trim_end_matches('/'), timestamp, ext)
                } else {
                    format!("results_{}.{}", timestamp, ext)
                };

                eprintln!("Uploading results to GCS bucket '{}' as '{}'...", upload.bucket, object_name);
                match upload_to_gcs(&upload.bucket, &object_name, output_data, upload.credential.as_deref()).await {
                    Ok(_) => {
                        eprintln!("Successfully uploaded test results to GCS.");
                        
                        if let Some(recent) = &upload.recent_summary {
                            let new_url = format!(
                                "{}/{}/{}",
                                recent.entrypoint.trim_end_matches('/'),
                                upload.bucket,
                                object_name
                            );
                            
                            let summary_object_name = if let Some(dir) = &recent.directory {
                                format!("{}/{}", dir.trim_end_matches('/'), recent.filename)
                            } else {
                                recent.filename.clone()
                            };

                            eprintln!("Updating recent summary file '{}'...", summary_object_name);
                            
                            let existing = read_from_gcs(&upload.bucket, &summary_object_name, upload.credential.as_deref()).await
                                .unwrap_or_else(|e| {
                                    eprintln!("Warning: failed to read existing summary: {}", e);
                                    String::new()
                                });

                            let re = Regex::new(r#"href="([^"]+)""#).unwrap();
                            let mut urls: Vec<String> = re.captures_iter(&existing)
                                .map(|cap| cap[1].to_string())
                                .collect();

                            urls.insert(0, new_url);
                            urls.truncate(recent.count);

                            let mut new_summary_content = String::new();
                            new_summary_content.push_str("<!DOCTYPE html>\n<html>\n<head>\n");
                            new_summary_content.push_str("  <meta charset=\"utf-8\">\n");
                            new_summary_content.push_str("  <title>Recent Test Results</title>\n");
                            new_summary_content.push_str("  <style>\n");
                            new_summary_content.push_str("    body { font-family: -apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, Helvetica, Arial, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; line-height: 1.6; }\n");
                            new_summary_content.push_str("    h1 { border-bottom: 2px solid #eaecef; padding-bottom: 0.3em; color: #24292e; }\n");
                            new_summary_content.push_str("    ul { list-style-type: none; padding: 0; }\n");
                            new_summary_content.push_str("    li { padding: 8px 12px; margin-bottom: 8px; background-color: #f6f8fa; border: 1px solid #e1e4e8; border-radius: 6px; }\n");
                            new_summary_content.push_str("    a { color: #0366d6; text-decoration: none; font-weight: 600; font-family: monospace; }\n");
                            new_summary_content.push_str("    a:hover { text-decoration: underline; }\n");
                            new_summary_content.push_str("  </style>\n");
                            new_summary_content.push_str("</head>\n<body>\n");
                            new_summary_content.push_str("  <h1>Recent Test Results</h1>\n");
                            new_summary_content.push_str("  <ul>\n");
                            for url in &urls {
                                let filename = url.split('/').last().unwrap_or(url);
                                new_summary_content.push_str(&format!("    <li><a href=\"{}\">{}</a></li>\n", url, filename));
                            }
                            new_summary_content.push_str("  </ul>\n");
                            new_summary_content.push_str("</body>\n</html>\n");

                            match upload_to_gcs(&upload.bucket, &summary_object_name, new_summary_content, upload.credential.as_deref()).await {
                                Ok(_) => eprintln!("Successfully updated recent summary in GCS."),
                                Err(e) => eprintln!("Failed to update recent summary in GCS: {}", e),
                            }
                        }
                    }
                    Err(e) => eprintln!("Failed to upload test results to GCS: {}", e),
                }
            }
        }
    }

    eprintln!(
        "Execution finished in {:.2?}. Total: {}, Succeeded: {}, Failed: {}",
        elapsed, total, succeeded, failed
    );

    if failed > 0 {
        std::process::exit(1);
    }

    Ok(())
}

async fn check_http(task: &Task, global_timeout: Duration, semaphore: Arc<Semaphore>) -> Result<()> {
    let target_raw = task.target.as_ref().ok_or(anyhow!("Missing target"))?;
    let target = target_raw.trim_matches('"');
    let timeout = task.timeout.as_deref().map(parse_duration).unwrap_or(global_timeout);
    
    let client = Client::builder()
        .timeout(timeout)
        .use_rustls_tls()
        .redirect(Policy::none())
        .build()?;
    
    let method = task.method.as_deref().unwrap_or("GET");
    let mut rb = match method {
        "POST" => client.post(target),
        "PUT" => client.put(target),
        _ => client.get(target),
    };

    if let Some(headers) = &task.headers {
        for (k, v) in headers {
            rb = rb.header(k, v);
        }
    }

    if let Some(body) = &task.body {
        rb = rb.body(body.clone());
    }

    let _permit = semaphore.acquire().await.unwrap();
    let resp = rb.send().await?;
    let status = resp.status().as_u16();
    let body_text = resp.text().await.unwrap_or_default();
    drop(_permit);

    if let Some(expect) = &task.expect {
        if let Some(expected_status) = expect.status {
            if status != expected_status {
                return Err(anyhow!("Status mismatch: expected {}, got {}", expected_status, status));
            }
        }
        if let Some(expected_bodies) = &expect.body {
            for b in expected_bodies {
                if !body_text.contains(b) {
                    return Err(anyhow!("Body mismatch: could not find '{}'", b));
                }
            }
        }
        if let Some(expected_json) = &expect.json {
            let v: serde_json::Value = serde_json::from_str(&body_text)
                .map_err(|e| anyhow!("Failed to parse JSON response: {} (Body: {})", e, body_text))?;
            for (path, expected_val) in expected_json {
                let found = select(&v, path).map_err(|_| anyhow!("JSONPath error at {}", path))?;
                if found.is_empty() || found[0].as_str() != Some(expected_val) {
                    let actual = found.get(0).map(|v| format!("{:?}", v)).unwrap_or_else(|| "null".to_string());
                    return Err(anyhow!("JSONPath mismatch at {}: expected {}, got {}", path, expected_val, actual));
                }
            }
        }
    }

    Ok(())
}

async fn check_tcp(task: &Task, global_timeout: Duration, semaphore: Arc<Semaphore>) -> Result<()> {
    let host = task.host.as_ref().or(task.target.as_ref()).ok_or(anyhow!("Missing host/target"))?;
    let timeout = task.timeout.as_deref().map(parse_duration).unwrap_or(global_timeout);
    
    let mut ports = Vec::new();
    if let Some(p) = task.port {
        ports.push(p);
    }
    if let Some(ps) = &task.ports {
        ports.extend(ps);
    }

    if ports.is_empty() {
        return Err(anyhow!("Missing port/ports"));
    }

    let resolver = get_resolver(&task.server).await?;
    let response = resolver.lookup_ip(host).await?;
    let ip = response.iter().next().ok_or(anyhow!("Could not resolve host"))?;

    let mut check_tasks = Vec::new();
    for port in ports {
        let addr = SocketAddr::from((ip, port));
        let sem = semaphore.clone();
        check_tasks.push(async move {
            let _permit = sem.acquire().await.unwrap();
            match tokio::time::timeout(timeout, TcpStream::connect(&addr)).await {
                Ok(Ok(_)) => Ok(()),
                _ => Err(anyhow!("Port {} is unreachable", port)),
            }
        });
    }

    let results = join_all(check_tasks).await;
    for res in results {
        res?;
    }

    Ok(())
}

async fn check_dns(task: &Task, semaphore: Arc<Semaphore>) -> Result<()> {
    let target = task.target.as_ref().ok_or(anyhow!("Missing target"))?;
    let _permit = semaphore.acquire().await.unwrap();
    let resolver = get_resolver(&task.server).await?;
    let response = resolver.lookup_ip(target).await?;
    drop(_permit);
    let ips: Vec<String> = response.iter().map(|ip| ip.to_string()).collect();
    
    if let Some(expected) = &task.expected_records {
        for e in expected {
            if !ips.contains(e) {
                return Err(anyhow!("DNS record {} not found in {:?}", e, ips));
            }
        }
    }

    if let Some(unexpected) = &task.unexpected_records {
        for u in unexpected {
            if ips.contains(u) {
                return Err(anyhow!("DNS record {} found in result (unexpected), resolved IPs: {:?}", u, ips));
            }
        }
    }

    Ok(())
}

async fn check_ssl(task: &Task, global_timeout: Duration, semaphore: Arc<Semaphore>) -> Result<()> {
    let target_raw = task.target.as_ref().ok_or(anyhow!("Missing target"))?;
    let target = target_raw.trim_matches('"');
    let timeout = task.timeout.as_deref().map(parse_duration).unwrap_or(global_timeout);
    
    let client = Client::builder()
        .timeout(timeout)
        .use_rustls_tls()
        .build()?;
    
    let url = if target.contains("://") {
        target.to_string()
    } else {
        format!("https://{}", target)
    };
    
    let _permit = semaphore.acquire().await.unwrap();
    client.get(&url).send().await?;
    Ok(())
}

async fn check_port_scan(task: &Task, semaphore: Arc<Semaphore>) -> Result<()> {
    let host = task.host.as_ref().or(task.target.as_ref()).ok_or(anyhow!("Missing host/target"))?;
    let mut ports_to_scan = Vec::new();

    if let Some(range_str) = &task.range {
        let parts: Vec<&str> = range_str.split('-').collect();
        if parts.len() == 2 {
            let start: u16 = parts[0].parse()?;
            let end: u16 = parts[1].parse()?;
            for p in start..=end {
                ports_to_scan.push(p);
            }
        }
    }

    if let Some(ports) = &task.ports {
        ports_to_scan.extend(ports);
    }

    if ports_to_scan.is_empty() {
        return Err(anyhow!("No ports specified for scan"));
    }

    let mut scan_tasks = Vec::new();

    let resolver = get_resolver(&task.server).await?;
    let response = resolver.lookup_ip(host).await?;
    let ip = response.iter().next().ok_or(anyhow!("Could not resolve host"))?;

    for &port in &ports_to_scan {
        let sem = semaphore.clone();
        scan_tasks.push(tokio::spawn(async move {
            let _permit = sem.acquire().await.unwrap();
            let addr = SocketAddr::from((ip, port));
            let timeout = Duration::from_millis(500); 
            match tokio::time::timeout(timeout, TcpStream::connect(&addr)).await {
                Ok(Ok(_)) => Some(port),
                _ => None,
            }
        }));
    }

    let results = join_all(scan_tasks).await;
    let open_ports: Vec<u16> = results.into_iter()
        .filter_map(|r| r.ok().flatten())
        .collect();

    if let Some(expect_open) = &task.expect_open {
        for p in expect_open {
            if !open_ports.contains(p) {
                return Err(anyhow!("Port {} is closed, but expected to be open", p));
            }
        }
    }

    if let Some(expect_closed) = &task.expect_closed {
        for p in expect_closed {
            if open_ports.contains(p) {
                return Err(anyhow!("Port {} is open, but expected to be closed", p));
            }
        }
    }

    Ok(())
}

async fn check_noindex(task: &Task, global_timeout: Duration, semaphore: Arc<Semaphore>) -> Result<()> {
    let target_raw = task.target.as_ref().ok_or(anyhow!("Missing target"))?;
    let target = target_raw.trim_matches('"');
    let timeout = task.timeout.as_deref().map(parse_duration).unwrap_or(global_timeout);

    let client = Client::builder()
        .timeout(timeout)
        .use_rustls_tls()
        .redirect(Policy::default())
        .build()?;

    let _permit = semaphore.acquire().await.unwrap();
    let resp = client.get(target).send().await?;
    
    // 1. Check HTTP header: X-Robots-Tag
    if let Some(robots_tag) = resp.headers().get("X-Robots-Tag") {
        if let Ok(val) = robots_tag.to_str() {
            if val.to_lowercase().contains("noindex") {
                return Ok(());
            }
        }
    }

    let body = resp.text().await?;
    drop(_permit);

    // 2. Check HTML Meta tag
    // Matches <meta name="robots" content="noindex"> and specific bots like googlebot
    let re = Regex::new(r#"(?i)<meta\s+[^>]*name=["'](?:robots|googlebot|bingbot|slurp|msnbot|teoma)["'][^>]*content=["'][^"']*noindex[^"']*["']"#).unwrap();
    
    if re.is_match(&body) {
        Ok(())
    } else {
        Err(anyhow!("No noindex tag found in headers or body"))
    }
}

async fn upload_to_gcs(
    bucket: &str,
    object_name: &str,
    data: String,
    credential_path: Option<&str>,
) -> Result<()> {
    let mut builder = Storage::builder();

    if let Some(path) = credential_path {
        let key_data = fs::read_to_string(path)
            .map_err(|e| anyhow!("Failed to read credential file at {}: {}", path, e))?;
        let key_json: serde_json::Value = serde_json::from_str(&key_data)
            .map_err(|e| anyhow!("Failed to parse credential JSON from {}: {}", path, e))?;
        
        let credentials = google_cloud_auth::credentials::service_account::Builder::new(key_json)
            .build()
            .map_err(|e| anyhow!("Failed to build credentials: {}", e))?;

        builder = builder.with_credentials(credentials);
    }

    let client = builder.build().await?;
    let bucket_path = format!("projects/_/buckets/{}", bucket);

    let content_type = if object_name.ends_with(".txt") {
        "text/plain"
    } else if object_name.ends_with(".json") {
        "application/json"
    } else if object_name.ends_with(".html") {
        "text/html"
    } else {
        "application/octet-stream"
    };

    client
        .write_object(&bucket_path, object_name, data)
        .set_content_type(content_type)
        .set_content_disposition("inline")
        .send_buffered()
        .await?;
    Ok(())
}

async fn read_from_gcs(
    bucket: &str,
    object_name: &str,
    credential_path: Option<&str>,
) -> Result<String> {
    let mut builder = Storage::builder();

    if let Some(path) = credential_path {
        let key_data = fs::read_to_string(path)
            .map_err(|e| anyhow!("Failed to read credential file at {}: {}", path, e))?;
        let key_json: serde_json::Value = serde_json::from_str(&key_data)
            .map_err(|e| anyhow!("Failed to parse credential JSON from {}: {}", path, e))?;
        
        let credentials = google_cloud_auth::credentials::service_account::Builder::new(key_json)
            .build()
            .map_err(|e| anyhow!("Failed to build credentials: {}", e))?;

        builder = builder.with_credentials(credentials);
    }

    let client = builder.build().await?;
    let bucket_path = format!("projects/_/buckets/{}", bucket);

    let mut resp = match client.read_object(&bucket_path, object_name).send().await {
        Ok(r) => r,
        Err(e) => {
            eprintln!("Info: could not read existing summary (might not exist yet): {}", e);
            return Ok(String::new());
        }
    };

    let mut contents = Vec::new();
    while let Some(chunk) = resp.next().await {
        match chunk {
            Ok(bytes) => contents.extend_from_slice(&bytes),
            Err(e) => return Err(anyhow!("Failed reading chunk: {}", e)),
        }
    }

    Ok(String::from_utf8(contents)?)
}


