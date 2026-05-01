use serde::Deserialize;
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

#[derive(Debug, Deserialize)]
struct Config {
    globals: Option<Globals>,
    tasks: Option<Vec<Task>>,
}

#[derive(Debug, Deserialize)]
struct Globals {
    timeout: Option<String>,
}

#[derive(Debug, Deserialize, Clone)]
struct Task {
    name: String,
    #[serde(rename = "type")]
    task_type: String,
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
    let args: Vec<String> = env::args().collect();
    let mut config_files = Vec::new();
    
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
            _ => i += 1,
        }
    }

    if config_files.is_empty() {
        eprintln!("Usage: {} --config-file <file.yaml> | --config-dir <dir>", args[0]);
        std::process::exit(1);
    }

    let mut all_tasks = Vec::new();
    let mut merged_timeout = None;

    for path in config_files {
        let data = fs::read_to_string(&path)
            .map_err(|e| anyhow!("Failed to read config file {}: {}", path, e))?;
        let config: Config = serde_yaml::from_str(&data)
            .map_err(|e| anyhow!("Failed to parse config file {}: {}", path, e))?;
        
        if let Some(globals) = config.globals {
            if let Some(timeout) = globals.timeout {
                merged_timeout = Some(timeout);
            }
        }
        
        if let Some(tasks) = config.tasks {
            all_tasks.extend(tasks);
        }
    }

    let timeout_str = merged_timeout.unwrap_or_else(|| "5s".to_string());
    let global_timeout = parse_duration(&timeout_str);

    println!("{:<30} {:<10} {:<10} {:<10}", "NAME", "TYPE", "STATUS", "LATENCY");
    println!("{}", "-".repeat(65));

    let semaphore = Arc::new(Semaphore::new(1000));
    let mut handles = Vec::new();

    for task in all_tasks {
        let sem = semaphore.clone();
        let handle = tokio::spawn(async move {
            let start = Instant::now();
            let result = match task.task_type.as_str() {
                "http" => check_http(&task, global_timeout, sem).await,
                "tcp" => check_tcp(&task, global_timeout, sem).await,
                "dns" => check_dns(&task, sem).await,
                "ssl" => check_ssl(&task, global_timeout, sem).await,
                "port_scan" => check_port_scan(&task, sem).await,
                "noindex" => check_noindex(&task, global_timeout, sem).await,
                _ => Err(anyhow!("Unknown task type: {}", task.task_type)),
            };
            let duration = start.elapsed();

            match result {
                Ok(_) => {
                    println!("{:<30} {:<10} \x1b[32m{:<10}\x1b[0m {:<10?}", task.name, task.task_type, "OK", duration);
                }
                Err(e) => {
                    println!("{:<30} {:<10} \x1b[31m{:<10}\x1b[0m {:<10?} Error: {}", task.name, task.task_type, "FAIL", duration, e);
                }
            }
        });
        handles.push(handle);
    }

    join_all(handles).await;

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
