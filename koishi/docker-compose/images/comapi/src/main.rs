use warp::Filter;
use pnet::datalink;
use std::process::Command;
use std::fs;
use std::env;

fn ok() -> String {
    "ok".to_string()
}

fn get_ipv6_by_interface(interface: &str) -> Option<String> {
    for iface in datalink::interfaces() {
        if iface.name == interface {
            for ip_network in iface.ips {
                if let std::net::IpAddr::V6(ipv6_addr) = ip_network.ip() {
                    return Some(ipv6_addr.to_string());
                }
            }
        }
    }
    None
}

fn get_interface_by_ipv4_prefix(prefix: &str) -> Option<String> {
    for iface in datalink::interfaces() {
        for ip_network in iface.ips {
            if ip_network.ip().to_string().starts_with(prefix) {
                return Some(iface.name);
            }
        }
    }
    None
}

fn get_ipv6_by_ipv4_prefix(prefix: &str) -> Option<String> {
    if let Some(interface) = get_interface_by_ipv4_prefix(prefix) {
        return get_ipv6_by_interface(interface.as_str());
    } else {
        return None;
    }
}

fn get_ipv6() -> String {
    let prefix = env::var("KOISHI_IPV4_PREFIX").expect("KOISHI_IPV4_PREFIX environment variable is required");
    if let Some(ipv6) = get_ipv6_by_ipv4_prefix(&prefix) {
        return ipv6;
    } else {
        return "".to_string();
    }
}

fn get_cpu_load() -> String {
    let output = Command::new("uptime")
        .output()
        .expect("failed to execute command");

    let output = String::from_utf8_lossy(&output.stdout);
    let parts: Vec<&str> = output.split("load average: ").collect();
    let loadstr = parts.get(1).unwrap_or(&"");
    let loadstrs: Vec<&str> = loadstr.split(",").collect();

    return loadstrs.get(0).unwrap_or(&"").trim().to_string();
}

fn get_mem_usage() -> String {
    let output = Command::new("free").arg("-m")
        .output()
        .expect("failed to execute command");

    let output = String::from_utf8_lossy(&output.stdout);

    let used_mb = output
                .lines()
                .find(|line| line.starts_with("Mem:"))
                .and_then(|line| line.split_whitespace().nth(2))
                .unwrap_or("0");

    let used_gb: f64 = used_mb.parse::<f64>().unwrap_or(0.0) / 1024.0;

    return format!("{:.1}G", used_gb);
}

fn get_top() -> String {
    let cpu = get_cpu_load();
    let mem = get_mem_usage();
    let temp = get_cpu_temp();

    return format!("{},{},{}", cpu, mem, temp);
}

fn get_cpu_temp() -> String {
    // Try to read from thermal zone 0 first
    let temp_path = "/sys/class/thermal/thermal_zone0/temp";
    match fs::read_to_string(temp_path) {
        Ok(temp_str) => {
            // Temperature is in millidegree Celsius, convert to Celsius
            if let Ok(temp) = temp_str.trim().parse::<f64>() {
                return format!("{}°C", (temp / 1000.0) as i32);
            }
        }
        Err(_) => {
            // If thermal_zone0 fails, try to find any thermal zone
            if let Ok(entries) = fs::read_dir("/sys/class/thermal") {
                for entry in entries.flatten() {
                    if entry.file_name().to_string_lossy().starts_with("thermal_zone") {
                        let temp_path = entry.path().join("temp");
                        if let Ok(temp_str) = fs::read_to_string(temp_path) {
                            if let Ok(temp) = temp_str.trim().parse::<f64>() {
                                return format!("{}°C", (temp / 1000.0) as i32);
                            }
                        }
                    }
                }
            }
        }
    }
    "N/A".to_string()
}

#[tokio::main]
async fn main() {
    if env::var("KOISHI_IPV4_PREFIX").is_err() {
        eprintln!("Error: KOISHI_IPV4_PREFIX environment variable is required");
        std::process::exit(1);
    }

    let health = warp::path("ok")
        .map(|| ok());

    let ipv6 = warp::path("ipv6")
        .map(|| get_ipv6());

    let top = warp::path("top")
        .map(|| get_top());

    let routes = health.or(ipv6).or(top);

    println!("Starting comapi ...");

    warp::serve(routes).run(([0, 0, 0, 0], 37900)).await;
}
