#![allow(non_snake_case)]

use std::io::{BufRead, BufReader, Write};
use std::process::{Command, Stdio};

use regex::Regex;
use walkdir::WalkDir;

fn findOSPATH() {
    let walker = WalkDir::new("/Volumes").max_depth(2).follow_links(true);
    for entry in walker.into_iter().filter_map(|e| e.ok()) {
        if !entry.path().is_dir() {
            continue;
        }
        let path_pwd = entry.path().to_str().unwrap_or("");
        if path_pwd.contains("Users") && !Regex::new(r#"(?i)\b数据|\bData|\bSystem|private"#).unwrap().is_match(path_pwd) {
            println!("{}", path_pwd);
        }
    }
}

fn main() {
}

fn findOSPATH1() {
    let output = Command::new("find")
        .arg("-L")
        .arg("/Volumes")
        .arg("-iname")
        .arg("Users")
        .arg("-type")
        .arg("d")
        .arg("-maxdepth")
        .arg("2")
        .arg("-follow")
        .stderr(Stdio::piped()) // 捕获 stderr 输出
        .stdout(Stdio::piped())
        .spawn()
        .expect("Failed to execute command");

    let stdout = output.stdout.expect("Failed to open stdout");
    let reader = BufReader::new(stdout);

    let stderr = output.stderr.expect("Failed to open stderr");
    let err_reader = BufReader::new(stderr);

    let mut has_no_such_file_error = false;

    for line in err_reader.lines() {
        let error_msg = line.expect("Failed to read error line");
        if error_msg.contains("No such file or directory") {
            has_no_such_file_error = true;
        }
    }

    if has_no_such_file_error {
        eprintln!("Command execution failed: No such file or directory");
    }

    let excluded_patterns = Regex::new(r#"(?i)\b数据|\bData|\bSystem|private"#).unwrap();

    for line in reader.lines() {
        let path = line.expect("Failed to read line");

        if !excluded_patterns.is_match(&path) {
            println!("{}", path);
        }
    }
}