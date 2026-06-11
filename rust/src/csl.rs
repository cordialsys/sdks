use crate::client::{Client, ClientError};
use crate::keyring::{Keyring, KeyringError};
use crate::types::*;
use serde_json::{Map, Value, json};
use std::str::FromStr;

#[derive(Debug, thiserror::Error)]
pub enum CslError {
    #[error("parse error on line {line}: {message}")]
    Parse { line: usize, message: String },
    #[error("client: {0}")]
    Client(#[from] ClientError),
    #[error("keyring: {0}")]
    Keyring(#[from] KeyringError),
    #[error("{0}")]
    Message(String),
}

#[derive(Debug, Clone, PartialEq)]
pub struct Program {
    pub commands: Vec<Command>,
}

#[derive(Debug, Clone, PartialEq)]
pub enum Command {
    Nop,
    Exit,
    Set { key: Vec<String>, value: String },
    Get { resource_name: String },
    List { resource_type: ResourceType },
    Delete { resource_name: String },
    Create(CreateCommand),
}

#[derive(Debug, Clone, PartialEq)]
pub struct CreateCommand {
    pub resource_type: ResourceType,
    pub variant: Option<String>,
    pub id: Option<String>,
    pub parent_id: Option<String>,
    pub fields: Object,
}

pub fn parse(source: &str) -> Result<Program, CslError> {
    let mut commands = Vec::new();
    for (idx, line) in source.lines().enumerate() {
        commands.push(parse_line(idx + 1, line)?);
    }
    Ok(Program { commands })
}

pub fn parse_line(line_no: usize, raw: &str) -> Result<Command, CslError> {
    let line = raw.trim();
    if line.is_empty() || line.starts_with('#') || line.starts_with("//") || line.starts_with("#!")
    {
        return Ok(Command::Nop);
    }
    let tokens = tokenize(line);
    if tokens.is_empty() {
        return Ok(Command::Nop);
    }
    match tokens[0].as_str() {
        "exit" => Ok(Command::Exit),
        "set" => parse_set(line_no, &tokens[1..]),
        "get" => parse_get(line_no, &tokens[1..]),
        "list" => parse_list(line_no, &tokens[1..]),
        "delete" => parse_delete(line_no, &tokens[1..]),
        "create" => parse_create(line_no, line, &tokens[1..]),
        keyword => {
            if let Ok(resource_type) = ResourceType::from_str(keyword) {
                if tokens.len() == 1 {
                    return Ok(Command::List { resource_type });
                }
                if tokens.len() >= 2 {
                    return Ok(Command::Get {
                        resource_name: format!("{}/{}", resource_type.path(), tokens[1]),
                    });
                }
            }
            Err(parse_error(
                line_no,
                format!("unrecognised command: {keyword}"),
            ))
        }
    }
}

pub struct Vm {
    pub client: Client,
    pub keyring: Option<Keyring>,
}

impl Vm {
    pub fn new(client: Client) -> Self {
        Self {
            client,
            keyring: None,
        }
    }

    pub fn with_keyring(mut self, keyring: Keyring) -> Self {
        self.keyring = Some(keyring);
        self
    }

    pub fn execute(&mut self, program: &Program) -> Result<(), CslError> {
        for command in &program.commands {
            self.execute_command(command)?;
        }
        Ok(())
    }

    pub fn execute_command(&mut self, command: &Command) -> Result<(), CslError> {
        match command {
            Command::Nop => Ok(()),
            Command::Exit => Err(CslError::Message("exit".to_string())),
            Command::Set { key, value } => self.execute_set(key, value),
            Command::Get { resource_name } => {
                let value = self.client.get_json(resource_name)?;
                println!(
                    "{}",
                    serde_json::to_string_pretty(&value).unwrap_or_else(|_| value.to_string())
                );
                Ok(())
            }
            Command::List { resource_type } => {
                let page = self.client.list(*resource_type, Default::default())?;
                println!(
                    "{}",
                    serde_json::to_string_pretty(&json!({ resource_type.path(): page.resources }))
                        .unwrap()
                );
                Ok(())
            }
            Command::Delete { resource_name } => {
                println!("{}", self.client.delete(resource_name)?);
                Ok(())
            }
            Command::Create(cmd) => {
                println!("{}", self.execute_create(cmd)?);
                Ok(())
            }
        }
    }

    pub fn execute_create(&self, cmd: &CreateCommand) -> Result<String, CslError> {
        match cmd.resource_type {
            ResourceType::Account => {
                let variant = parse_required_variant::<AccountVariant>(cmd)?;
                self.client
                    .create_account(
                        cmd.id.as_deref(),
                        &CreateAccountRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::Address => {
                let variant = parse_required_variant::<AddressVariant>(cmd)?;
                let parent_id = cmd.parent_id.as_deref().ok_or_else(|| {
                    CslError::Message("address create requires parent chain id".to_string())
                })?;
                self.client
                    .create_address(
                        parent_id,
                        cmd.id.as_deref(),
                        &CreateAddressRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::AccessRule
            | ResourceType::TransferRule
            | ResourceType::CallRule
            | ResourceType::StakingRule => {
                let req = CreateRuleRequest {
                    variant: parse_required_variant::<RuleVariant>(cmd)?,
                    metadata: Metadata::default(),
                    data: cmd.fields.clone(),
                };
                match cmd.resource_type {
                    ResourceType::AccessRule => {
                        self.client.create_access_rule(cmd.id.as_deref(), &req)
                    }
                    ResourceType::TransferRule => {
                        self.client.create_transfer_rule(cmd.id.as_deref(), &req)
                    }
                    ResourceType::CallRule => self.client.create_call_rule(cmd.id.as_deref(), &req),
                    ResourceType::StakingRule => {
                        self.client.create_staking_rule(cmd.id.as_deref(), &req)
                    }
                    _ => unreachable!(),
                }
                .map_err(Into::into)
            }
            ResourceType::Key => {
                let variant = parse_required_variant::<KeyVariant>(cmd)?;
                self.client
                    .create_key(
                        cmd.id.as_deref(),
                        &CreateKeyRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::Credential => {
                let variant = parse_required_variant::<CredentialVariant>(cmd)?;
                let parent_id = cmd.parent_id.as_deref().ok_or_else(|| {
                    CslError::Message("credential create requires parent user id".to_string())
                })?;
                self.client
                    .create_credential(
                        parent_id,
                        cmd.id.as_deref(),
                        &CreateCredentialRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::User => {
                let variant = parse_required_variant::<UserVariant>(cmd)?;
                self.client
                    .create_user(
                        cmd.id.as_deref(),
                        &CreateUserRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::Asset => {
                let variant = parse_required_variant::<AssetVariant>(cmd)?;
                let parent_id = cmd.parent_id.as_deref().ok_or_else(|| {
                    CslError::Message("asset create requires parent chain id".to_string())
                })?;
                self.client
                    .create_asset(
                        parent_id,
                        cmd.id.as_deref(),
                        &CreateAssetRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::Chain => {
                let variant = parse_required_variant::<ChainVariant>(cmd)?;
                self.client
                    .create_chain(
                        cmd.id.as_deref(),
                        &CreateChainRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            ResourceType::Staking => {
                let variant = parse_required_variant::<StakingVariant>(cmd)?;
                self.client
                    .create_staking(
                        cmd.id.as_deref(),
                        &CreateStakingRequest {
                            variant,
                            metadata: Metadata::default(),
                            data: cmd.fields.clone(),
                        },
                    )
                    .map_err(Into::into)
            }
            other => self
                .client
                .create_resource(
                    other,
                    cmd.id.as_deref(),
                    cmd.parent_id.as_deref(),
                    &CreateResourceRequest {
                        metadata: Metadata::default(),
                        data: cmd.fields.clone(),
                    },
                )
                .map_err(Into::into),
        }
    }

    fn execute_set(&mut self, key: &[String], value: &str) -> Result<(), CslError> {
        match key.join(".").as_str() {
            "treasury" | "treasury.id" => {
                self.client.set_treasury_id(value);
                Ok(())
            }
            "sign.with" => {
                let keyring = self
                    .keyring
                    .as_ref()
                    .ok_or_else(|| CslError::Message("no keyring configured".to_string()))?;
                let identity = keyring.get_key(value)?;
                self.client.set_signer(identity);
                Ok(())
            }
            _ => Ok(()),
        }
    }
}

fn parse_required_variant<T>(cmd: &CreateCommand) -> Result<T, CslError>
where
    T: FromStr<Err = String>,
{
    let variant = cmd.variant.as_deref().ok_or_else(|| {
        CslError::Message(format!(
            "{} create requires a typed variant",
            cmd.resource_type
        ))
    })?;
    T::from_str(variant).map_err(CslError::Message)
}

fn parse_set(line_no: usize, tokens: &[String]) -> Result<Command, CslError> {
    if tokens.len() < 2 {
        return Err(parse_error(line_no, "set requires key and value"));
    }
    Ok(Command::Set {
        key: tokens[0].split('.').map(str::to_string).collect(),
        value: tokens[1].clone(),
    })
}

fn parse_get(line_no: usize, tokens: &[String]) -> Result<Command, CslError> {
    if tokens.is_empty() {
        return Err(parse_error(line_no, "get requires a resource name"));
    }
    let name = if tokens.len() >= 2 {
        ResourceType::from_str(&tokens[0])
            .map(|rt| format!("{}/{}", rt.path(), tokens[1]))
            .unwrap_or_else(|_| tokens[0].clone())
    } else {
        tokens[0].clone()
    };
    Ok(Command::Get {
        resource_name: name,
    })
}

fn parse_list(line_no: usize, tokens: &[String]) -> Result<Command, CslError> {
    if tokens.is_empty() {
        return Err(parse_error(line_no, "list requires a resource type"));
    }
    Ok(Command::List {
        resource_type: ResourceType::from_str(&tokens.join(" "))
            .map_err(|message| parse_error(line_no, message))?,
    })
}

fn parse_delete(line_no: usize, tokens: &[String]) -> Result<Command, CslError> {
    if tokens.is_empty() {
        return Err(parse_error(line_no, "delete requires a resource name"));
    }
    Ok(Command::Delete {
        resource_name: tokens[0].clone(),
    })
}

fn parse_create(line_no: usize, line: &str, tokens: &[String]) -> Result<Command, CslError> {
    if tokens.is_empty() {
        return Err(parse_error(line_no, "create requires a resource type"));
    }
    let fields = parse_fields(line_no, line)?;
    let mut head: Vec<&str> = tokens
        .iter()
        .take_while(|token| token.as_str() != "{")
        .map(String::as_str)
        .collect();
    if head.last() == Some(&"with") {
        head.pop();
    }

    let mut parent_id = None;
    if let Some(pos) = head
        .iter()
        .position(|token| *token == "for" || *token == "under")
    {
        if pos + 1 < head.len() {
            parent_id = Some(head[pos + 1].to_string());
        }
        head.truncate(pos);
    }

    let mut id = None;
    if let Some(pos) = head
        .iter()
        .position(|token| *token == "named" || *token == "id")
    {
        if pos + 1 < head.len() {
            id = Some(head[pos + 1].to_string());
        }
        head.truncate(pos);
    }

    let (variant, resource_words) = match head.as_slice() {
        [resource] => (None, vec![*resource]),
        [first, second] if ResourceType::from_str(&format!("{first} {second}")).is_ok() => {
            (None, vec![*first, *second])
        }
        [variant, resource] => (Some((*variant).to_string()), vec![*resource]),
        [variant, first, second] => (Some((*variant).to_string()), vec![*first, *second]),
        _ => {
            return Err(parse_error(
                line_no,
                format!("cannot parse create target: {}", head.join(" ")),
            ));
        }
    };
    let resource_type = ResourceType::from_str(&resource_words.join(" "))
        .map_err(|message| parse_error(line_no, message))?;
    Ok(Command::Create(CreateCommand {
        resource_type,
        variant,
        id,
        parent_id,
        fields,
    }))
}

fn parse_fields(line_no: usize, line: &str) -> Result<Object, CslError> {
    let Some(start) = line.find('{') else {
        return Ok(Map::new());
    };
    let Some(end) = line.rfind('}') else {
        return Err(parse_error(line_no, "missing closing }"));
    };
    let body = line[start + 1..end].trim();
    let mut fields = Map::new();
    if body.is_empty() {
        return Ok(fields);
    }
    for part in split_top_level(body, ',') {
        let (key, value) = part
            .split_once('=')
            .ok_or_else(|| parse_error(line_no, format!("invalid field: {part}")))?;
        fields.insert(key.trim().to_string(), parse_value(value.trim()));
    }
    Ok(fields)
}

fn parse_value(raw: &str) -> Value {
    let raw = raw.trim();
    if raw.starts_with('"') && raw.ends_with('"') && raw.len() >= 2 {
        Value::String(raw[1..raw.len() - 1].to_string())
    } else if raw == "true" {
        Value::Bool(true)
    } else if raw == "false" {
        Value::Bool(false)
    } else if let Ok(n) = raw.parse::<i64>() {
        Value::Number(n.into())
    } else {
        Value::String(raw.to_string())
    }
}

fn tokenize(line: &str) -> Vec<String> {
    let mut out = Vec::new();
    let mut buf = String::new();
    let mut in_string = false;
    for ch in line.chars() {
        match ch {
            '"' => {
                in_string = !in_string;
                buf.push(ch);
            }
            ' ' | '\t' if !in_string => {
                if !buf.is_empty() {
                    out.push(unquote(&buf));
                    buf.clear();
                }
            }
            '{' | '}' | ',' if !in_string => {
                if !buf.is_empty() {
                    out.push(unquote(&buf));
                    buf.clear();
                }
                out.push(ch.to_string());
            }
            _ => buf.push(ch),
        }
    }
    if !buf.is_empty() {
        out.push(unquote(&buf));
    }
    out
}

fn split_top_level(input: &str, sep: char) -> Vec<&str> {
    let mut parts = Vec::new();
    let mut start = 0;
    let mut in_string = false;
    for (idx, ch) in input.char_indices() {
        if ch == '"' {
            in_string = !in_string;
        } else if ch == sep && !in_string {
            parts.push(input[start..idx].trim());
            start = idx + ch.len_utf8();
        }
    }
    parts.push(input[start..].trim());
    parts
}

fn unquote(value: &str) -> String {
    if value.starts_with('"') && value.ends_with('"') && value.len() >= 2 {
        value[1..value.len() - 1].to_string()
    } else {
        value.to_string()
    }
}

fn parse_error(line: usize, message: impl Into<String>) -> CslError {
    CslError::Parse {
        line,
        message: message.into(),
    }
}
