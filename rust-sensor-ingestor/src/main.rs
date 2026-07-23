use redis::Commands;
use std::env;
use std::fs::{OpenOptions, File};
use std::io::{Write, BufRead, BufReader};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

const QUEUE_FILE: &str = "offline_queue.txt";
const CPP_ENGINE_PATH: &str = "/mnt/d/IMPORTANT/Projects/Crowd Safety System/cpp-risk-engine/risk_engine";

fn now_epoch() -> i64 {
    SystemTime::now().duration_since(UNIX_EPOCH).unwrap().as_secs() as i64
}

// Calls your compiled C++ program and reads back "RiskLevel,Eta"
fn call_cpp_engine(current: i32, previous: i32, capacity: i32, minutes_elapsed: i32) -> (String, i32) {
    let output = Command::new(CPP_ENGINE_PATH)
        .arg(current.to_string())
        .arg(previous.to_string())
        .arg(capacity.to_string())
        .arg(minutes_elapsed.to_string())
        .output();

    match output {
        Ok(out) => {
            let text = String::from_utf8_lossy(&out.stdout).trim().to_string();
            let parts: Vec<&str> = text.split(',').collect();
            if parts.len() == 2 {
                (parts[0].to_string(), parts[1].parse().unwrap_or(-1))
            } else {
                ("Safe".to_string(), -1)
            }
        }
        Err(_) => ("Safe".to_string(), -1),
    }
}

// Does the actual work: updates Redis, calls C++, saves history, publishes live update
fn process_update(con: &mut redis::Connection, zone_id: &str, new_count: i32) -> redis::RedisResult<()> {
    let key = format!("zone:{}", zone_id);

    let prev_count: i32 = con.hget(&key, "current_count").unwrap_or(new_count);
    let last_updated: i64 = con.hget(&key, "last_updated").unwrap_or_else(|_| now_epoch());
    let capacity: i32 = con.hget(&key, "capacity").unwrap_or(500);

    let now = now_epoch();
    let mut minutes_elapsed = ((now - last_updated) as f64 / 60.0).round() as i32;
    if minutes_elapsed <= 0 {
        minutes_elapsed = 1;
    }

    let (risk_level, eta) = call_cpp_engine(new_count, prev_count, capacity, minutes_elapsed);

    let _: () = con.hset(&key, "current_count", new_count)?;
    let _: () = con.hset(&key, "risk_level", &risk_level)?;
    let _: () = con.hset(&key, "eta_danger", eta)?;
    let _: () = con.hset(&key, "last_updated", now)?;

    // Save to history (for analytics later) - member is "timestamp:count", score is timestamp
    let history_key = format!("zone:{}:history", zone_id);
    let member = format!("{}:{}", now, new_count);
    let _: () = con.zadd(&history_key, member, now)?;

    // Announce live update
    let channel = format!("zone_updates:{}", zone_id);
    let message = format!("{},{},{}", risk_level, eta, new_count);
    let _: () = con.publish(&channel, message)?;

    println!("✅ Zone {} => count={}, risk={}, eta={}min", zone_id, new_count, risk_level, eta);

    Ok(())
}

// If Redis is down, save the update to a local file instead of losing it
fn queue_offline(zone_id: &str, new_count: i32) {
    let mut file = OpenOptions::new().create(true).append(true).open(QUEUE_FILE).unwrap();
    writeln!(file, "{},{}", zone_id, new_count).unwrap();
    println!("⚠️  Redis unreachable — saved update offline for zone {}", zone_id);
}

// Every time we run, first check if there are old queued updates waiting to sync
fn process_queued_updates(con: &mut redis::Connection) {
    if let Ok(file) = File::open(QUEUE_FILE) {
        let reader = BufReader::new(file);
        let mut synced = 0;
        for line in reader.lines().flatten() {
            let parts: Vec<&str> = line.split(',').collect();
            if parts.len() == 2 {
                if let Ok(count) = parts[1].parse::<i32>() {
                    if process_update(con, parts[0], count).is_ok() {
                        synced += 1;
                    }
                }
            }
        }
        if synced > 0 {
            println!("📤 Synced {} previously queued offline update(s)", synced);
        }
        let _ = std::fs::remove_file(QUEUE_FILE);
    }
}

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() != 3 {
        eprintln!("Usage: sensor_ingestor <zone_id> <new_count>");
        std::process::exit(1);
    }

    let zone_id = &args[1];
    let new_count: i32 = args[2].parse().expect("count must be a number");

    let client = redis::Client::open("redis://127.0.0.1/").unwrap();

    match client.get_connection() {
        Ok(mut con) => {
            process_queued_updates(&mut con);
            if let Err(e) = process_update(&mut con, zone_id, new_count) {
                eprintln!("Redis error: {}", e);
                queue_offline(zone_id, new_count);
            }
        }
        Err(_) => {
            queue_offline(zone_id, new_count);
        }
    }
}
