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

fn parse_sensors_output(stdout: &str) -> Option<String> {
    for line in stdout.lines() {
        if line.contains("Tctl:") {
            if let Some(pos) = line.find(':') {
                let temp_val = line[pos + 1..].trim();
                if let Some(first_word) = temp_val.split_whitespace().next() {
                    let clean_temp: String = first_word.chars()
                        .filter(|c| c.is_ascii_digit() || *c == '.' || *c == '-' || *c == '+')
                        .collect();
                    if let Ok(val) = clean_temp.parse::<f64>() {
                        return Some(format!("{}°C", val.round() as i32));
                    }
                }
            }
        }
    }
    None
}

fn get_cpu_temp_miniba() -> String {
    if let Ok(output) = Command::new("sensors").output() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        if let Some(temp) = parse_sensors_output(&stdout) {
            return temp;
        }
    }
    "N/A".to_string()
}

fn get_cpu_temp_common() -> String {
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

fn get_cpu_temp() -> String {
    if let Ok(hostname) = fs::read_to_string("/proc/sys/kernel/hostname") {
        if hostname.trim() == "miniba" {
            return get_cpu_temp_miniba();
        }
    }
    get_cpu_temp_common()
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_sensors_output() {
        let sample_output = r#"
amdgpu-pci-0700
Adapter: PCI adapter
vddgfx:        1.29 V
vddnb:       849.00 mV
edge:         +47.0°C
PPT:          25.14 W

k10temp-pci-00c3
Adapter: PCI adapter
Tctl:         +52.4°C
Tccd1:        +44.4°C

nvme-pci-0100
Adapter: PCI adapter
Composite:    +38.9°C  (low  = -273.1°C, high = +89.8°C)
                       (crit = +94.8°C)
Sensor 1:     +38.9°C  (low  = -273.1°C, high = +65261.8°C)
Sensor 2:     +37.9°C  (low  = -273.1°C, high = +65261.8°C)

iwlwifi_1-virtual-0
Adapter: Virtual device
temp1:            N/A

nvme-pci-0300
Adapter: PCI adapter
Composite:    +33.9°C  (low  = -273.1°C, high = +89.8°C)
                       (crit = +94.8°C)
Sensor 1:     +33.9°C  (low  = -273.1°C, high = +65261.8°C)
Sensor 2:     +33.9°C  (low  = -273.1°C, high = +65261.8°C)
"#;
        assert_eq!(parse_sensors_output(sample_output), Some("52°C".to_string()));
    }
}
