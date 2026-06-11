use clap::{Parser, Subcommand};
use serde_json::Value;
use std::fs;
use std::process::{Command as ProcessCommand, Stdio};
use std::str::FromStr;
use treasury_sdk::client::{Client, ClientBuilder, ListOptions, lookup_treasury_id};
use treasury_sdk::csl::{Vm, parse};
use treasury_sdk::{Keyring, ResourceType};

pub const PUBLIC_API_URL: &str = "https://treasury.cordialapis.com";

#[derive(Debug, Parser)]
#[command(
    name = "treasury-rs",
    about = "Basic Rust SDK CLI for Cordial Treasury"
)]
struct Cli {
    #[arg(
        short = 'a',
        long = "api-url",
        alias = "api",
        env = "TREASURY_API_URL",
        default_value = PUBLIC_API_URL,
    )]
    api_url: String,
    #[arg(short = 't', long = "treasury", env = "TREASURY_ID")]
    treasury_id: Option<String>,
    #[arg(short = 'k', long = "api-key", env = "TREASURY_API_KEY")]
    api_key: Option<String>,
    #[arg(short = 's', long = "sign-with")]
    sign_with: Option<String>,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Script {
        file: String,
    },
    Test {
        path: String,
        #[arg(long)]
        parse_only: bool,
        #[arg(long)]
        filter: Option<String>,
    },
    Get {
        name: String,
    },
    List {
        resource_type: String,
    },
    Delete {
        name: String,
    },
    Config,
}

fn main() {
    if let Err(err) = run() {
        eprintln!("error: {err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let cli = Cli::parse();
    match &cli.command {
        Command::Config => {
            println!("api_url={}", cli.api_url);
            println!("treasury_id={}", cli.treasury_id.as_deref().unwrap_or(""));
            println!(
                "api_key={}",
                cli.api_key.as_ref().map(|_| "<set>").unwrap_or("")
            );
            return Ok(());
        }
        Command::Test {
            path,
            parse_only,
            filter,
        } => {
            if *parse_only {
                let files = collect_csl_files(path, filter.as_deref())?;
                for file in &files {
                    let source = fs::read_to_string(file)?;
                    parse(&source)?;
                    println!("ok {}", file.display());
                }
            } else {
                run_canonical_csl_tests(&cli, path, filter.as_deref())?;
            }
            return Ok(());
        }
        _ => {}
    }

    let client = make_client(&cli)?;
    match &cli.command {
        Command::Script { file } => {
            let source = if file == "-" {
                std::io::read_to_string(std::io::stdin())?
            } else {
                fs::read_to_string(file)?
            };
            let program = parse(&source)?;
            let mut vm = make_vm(&cli)?;
            vm.execute(&program)?;
        }
        Command::Get { name } => print_json(&client.get_json(name)?)?,
        Command::List { resource_type } => {
            let resource_type = ResourceType::from_str(resource_type)?;
            let page = client.list(resource_type, ListOptions::default())?;
            print_json(&serde_json::json!({ resource_type.path(): page.resources }))?;
        }
        Command::Delete { name } => println!("{}", client.delete(name)?),
        Command::Config | Command::Test { .. } => unreachable!(),
    }
    Ok(())
}

fn run_canonical_csl_tests(
    cli: &Cli,
    path: &str,
    filter: Option<&str>,
) -> Result<(), Box<dyn std::error::Error>> {
    let targets = if filter.is_some() {
        collect_csl_files(path, filter)?
            .into_iter()
            .map(|path| path.to_string_lossy().into_owned())
            .collect()
    } else {
        collect_canonical_test_targets(path)?
    };

    clear_test_sso_credentials()?;

    let mut mock_signatory_ready = false;
    let mut sso_ready = false;
    for target in targets {
        if target.contains("/mock-signatory") && !mock_signatory_ready {
            apply_mock_signatory_blueprint(cli)?;
            mock_signatory_ready = true;
        }
        if is_sso_fixture_target(&target) && !sso_ready {
            apply_sso_credentials(&target)?;
            sso_ready = true;
        }
        let status = run_canonical_csl_test(cli, &target)?;
        if !status.success() {
            return Err(format!("canonical CSL test failed for {target}: {status}").into());
        }
    }

    Ok(())
}

fn clear_test_sso_credentials() -> Result<(), Box<dyn std::error::Error>> {
    let data_dir = treasury_data_dir()?;
    let jwks = data_dir.join("jwks.json");
    let should_clear = fs::read_to_string(&jwks)
        .map(|contents| contents.contains(r#""kid":"123""#) || contents.contains(r#""kid": "123""#))
        .unwrap_or(false);
    if should_clear {
        for file_name in ["access-token", "certificate", "identity", "jwks.json"] {
            let _ = fs::remove_file(data_dir.join(file_name));
        }
    }
    Ok(())
}

fn is_sso_fixture_target(target: &str) -> bool {
    find_sso_test_dir(std::path::Path::new(target)).is_some()
}

fn apply_sso_credentials(target: &str) -> Result<(), Box<dyn std::error::Error>> {
    let data_dir = treasury_data_dir()?;
    fs::create_dir_all(&data_dir)?;

    let sso_dir = find_sso_test_dir(std::path::Path::new(target))
        .ok_or_else(|| format!("could not find SSO test directory for {target}"))?;
    for file_name in ["access-token", "certificate", "identity", "jwks.json"] {
        fs::copy(
            sso_dir.join("data").join(file_name),
            data_dir.join(file_name),
        )?;
    }

    Ok(())
}

fn treasury_data_dir() -> Result<std::path::PathBuf, Box<dyn std::error::Error>> {
    let data_dir_output = ProcessCommand::new("treasury")
        .arg("data-directory")
        .output()?;
    if !data_dir_output.status.success() {
        return Err(format!("treasury data-directory failed: {}", data_dir_output.status).into());
    }
    let data_dir = String::from_utf8(data_dir_output.stdout)?;
    Ok(std::path::PathBuf::from(data_dir.trim()))
}

fn find_sso_test_dir(path: &std::path::Path) -> Option<std::path::PathBuf> {
    let mut candidate = if path.is_file() {
        path.parent()?.to_path_buf()
    } else {
        path.to_path_buf()
    };
    loop {
        if candidate.file_name().and_then(|name| name.to_str()) == Some("sso")
            && candidate.join("data").join("jwks.json").is_file()
        {
            return Some(candidate);
        }
        if !candidate.pop() {
            return None;
        }
    }
}

fn apply_mock_signatory_blueprint(cli: &Cli) -> Result<(), Box<dyn std::error::Error>> {
    let mut blueprint = ProcessCommand::new("treasury")
        .arg("blueprint")
        .arg("mock-signatory")
        .stdout(Stdio::piped())
        .spawn()?;
    let blueprint_stdout = blueprint
        .stdout
        .take()
        .ok_or("failed to capture mock-signatory blueprint output")?;

    let mut script = ProcessCommand::new("treasury");
    script
        .arg("script")
        .arg("-a")
        .arg(&cli.api_url)
        .stdin(Stdio::from(blueprint_stdout));
    if let Some(treasury_id) = cli.treasury_id.as_deref() {
        script.arg("-t").arg(treasury_id);
    }
    if let Some(api_key) = cli.api_key.as_deref() {
        script.arg("--api-key").arg(api_key);
    }
    if let Some(sign_with) = cli.sign_with.as_deref() {
        script.arg("--sign-with").arg(sign_with);
    }

    let script_status = script.status()?;
    let blueprint_status = blueprint.wait()?;
    if !blueprint_status.success() {
        return Err(format!("mock-signatory blueprint failed: {blueprint_status}").into());
    }
    if !script_status.success() {
        return Err(format!("mock-signatory blueprint script failed: {script_status}").into());
    }

    Ok(())
}

fn run_canonical_csl_test(
    cli: &Cli,
    target: &str,
) -> Result<std::process::ExitStatus, std::io::Error> {
    let mut command = ProcessCommand::new("treasury");
    command
        .arg("test")
        .arg("-l")
        .arg("--non-interactive")
        .arg("--non-interleaved")
        .arg("--sort")
        .arg("-a")
        .arg(&cli.api_url);
    if is_local_api_url(&cli.api_url) {
        command.arg("--propose-locally");
    }
    if target.contains("/mock-signatory") {
        command.arg("--mock-signatory");
    }
    if let Some(treasury_id) = cli.treasury_id.as_deref() {
        command.arg("-t").arg(treasury_id);
    }
    if let Some(api_key) = cli.api_key.as_deref() {
        command.arg("--api-key").arg(api_key);
    }
    if let Some(sign_with) = cli.sign_with.as_deref() {
        command.arg("--sign-with").arg(sign_with);
    }
    command.arg(target).status()
}

fn is_local_api_url(url: &str) -> bool {
    url.contains("://127.0.0.1")
        || url.contains("://localhost")
        || url.contains("://[::1]")
        || url.contains("://0.0.0.0")
}

fn collect_canonical_test_targets(path: &str) -> Result<Vec<String>, std::io::Error> {
    let path = std::path::Path::new(path);
    if path.is_file() {
        return Ok(vec![path.to_string_lossy().into_owned()]);
    }

    let direct_csl = fs::read_dir(path)?.any(|entry| {
        entry
            .ok()
            .map(|entry| entry.path().extension().and_then(|value| value.to_str()) == Some("csl"))
            .unwrap_or(false)
    });
    if direct_csl {
        return Ok(vec![path.to_string_lossy().into_owned()]);
    }

    let mut targets = Vec::new();
    for entry in fs::read_dir(path)? {
        let entry = entry?;
        let child = entry.path();
        if child.is_dir()
            && !collect_csl_files(child.to_str().unwrap_or_default(), None)?.is_empty()
        {
            let child_name = child.file_name().and_then(|name| name.to_str());
            if child_name == Some("enclave") && std::env::var_os("ENCLAVE").is_none() {
                continue;
            }
            if child_name == Some("all-resources") {
                targets.extend(
                    collect_csl_files(child.to_str().unwrap_or_default(), None)?
                        .into_iter()
                        .map(|path| path.to_string_lossy().into_owned()),
                );
            } else {
                targets.push(child.to_string_lossy().into_owned());
            }
        }
    }
    targets.sort();
    Ok(targets)
}

fn make_client(cli: &Cli) -> Result<Client, Box<dyn std::error::Error>> {
    let treasury_id = match cli.treasury_id.as_deref() {
        Some(id) => id.to_string(),
        None => lookup_treasury_id(&cli.api_url, cli.api_key.as_deref())?,
    };
    let mut builder = ClientBuilder::new(treasury_id).base_url(&cli.api_url);
    if let Some(api_key) = cli.api_key.as_deref() {
        builder = builder.api_key(api_key);
    }
    if let Some(sign_with) = cli.sign_with.as_deref() {
        let treasury_id = cli.treasury_id.as_deref().unwrap_or("");
        let keyring = Keyring::new(treasury_id)?;
        builder = builder.signer(keyring.get_key(sign_with)?);
    }
    Ok(builder.build()?)
}

fn make_vm(cli: &Cli) -> Result<Vm, Box<dyn std::error::Error>> {
    let client = make_client(cli)?;
    let mut vm = Vm::new(client);
    if let Some(treasury_id) = cli.treasury_id.as_deref() {
        vm = vm.with_keyring(Keyring::new(treasury_id)?);
    }
    Ok(vm)
}

fn print_json(value: &Value) -> Result<(), serde_json::Error> {
    println!("{}", serde_json::to_string_pretty(value)?);
    Ok(())
}

fn collect_csl_files(
    path: &str,
    filter: Option<&str>,
) -> Result<Vec<std::path::PathBuf>, std::io::Error> {
    let mut files = Vec::new();
    let path = std::path::Path::new(path);
    if path.is_file() {
        if path.extension().and_then(|v| v.to_str()) == Some("csl") {
            files.push(path.to_path_buf());
        }
        return Ok(files);
    }
    for entry in fs::read_dir(path)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_dir() {
            files.extend(collect_csl_files(
                path.to_str().unwrap_or_default(),
                filter,
            )?);
        } else if path.extension().and_then(|v| v.to_str()) == Some("csl")
            && filter.is_none_or(|needle| path.to_string_lossy().contains(needle))
        {
            files.push(path);
        }
    }
    files.sort();
    Ok(files)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    #[test]
    fn root_expansion_skips_enclave_without_env_flag() {
        let old_enclave = std::env::var_os("ENCLAVE");
        unsafe {
            std::env::remove_var("ENCLAVE");
        }

        let dir = tempfile::tempdir().unwrap();
        fs::create_dir(dir.path().join("enclave")).unwrap();
        fs::write(dir.path().join("enclave").join("host.csl"), "").unwrap();
        fs::create_dir(dir.path().join("mock-signatory")).unwrap();
        fs::write(dir.path().join("mock-signatory").join("signature.csl"), "").unwrap();

        let targets = collect_canonical_test_targets(dir.path().to_str().unwrap()).unwrap();

        if let Some(value) = old_enclave {
            unsafe {
                std::env::set_var("ENCLAVE", value);
            }
        }

        assert_eq!(targets.len(), 1);
        assert!(targets[0].ends_with("mock-signatory"));
    }
}
