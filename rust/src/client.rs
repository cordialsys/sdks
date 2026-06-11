use crate::signing::{Signer, SigningError, sign_request};
use crate::types::*;
use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64;
use reqwest::Url;
use reqwest::blocking::Client as HttpClient;
use reqwest::header::{ACCEPT, AUTHORIZATION, CONTENT_TYPE, HeaderMap, HeaderValue};
use serde::Serialize;
use serde::de::DeserializeOwned;
use serde_json::{Value, json};
use std::sync::Arc;
use std::time::Duration;

pub const DEFAULT_BASE_URL: &str = "https://treasury.cordialapis.com/";

#[derive(Debug, Clone, Default)]
pub struct ListOptions {
    pub parent_id: Option<String>,
    pub filter: Option<String>,
    pub order_by: Option<String>,
    pub page_size: Option<u32>,
    pub page_token: Option<String>,
    pub show_deleted: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct ListResponse {
    pub resources: Vec<Value>,
    pub next_page_token: Option<String>,
    pub total_size: Option<u64>,
}

#[derive(Debug, thiserror::Error)]
pub enum ClientError {
    #[error("treasury id is required")]
    MissingTreasuryId,
    #[error("api key is required when using default Treasury API base URL {0}")]
    MissingApiKey(&'static str),
    #[error("invalid url: {0}")]
    Url(#[from] url::ParseError),
    #[error("http: {0}")]
    Http(#[from] reqwest::Error),
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    #[error("header: {0}")]
    Header(#[from] reqwest::header::InvalidHeaderValue),
    #[error("signing: {0}")]
    Signing(#[from] SigningError),
    #[error("treasury API error (HTTP {status_code}, status {status}): {message}")]
    Api {
        status_code: u16,
        code: Value,
        status: String,
        message: String,
        details: Vec<Value>,
    },
    #[error("{0}")]
    Message(String),
}

#[derive(Clone)]
pub struct Client {
    base_url: Url,
    treasury_id: String,
    host_id: Option<String>,
    api_key: Option<String>,
    http: HttpClient,
    signer: Option<Arc<dyn Signer>>,
}

impl Client {
    pub fn builder(treasury_id: impl Into<String>) -> ClientBuilder {
        ClientBuilder::new(treasury_id)
    }

    pub fn new(treasury_id: impl Into<String>) -> Result<Self, ClientError> {
        Self::builder(treasury_id).build()
    }

    pub fn base_url(&self) -> &Url {
        &self.base_url
    }

    pub fn treasury_id(&self) -> &str {
        &self.treasury_id
    }

    pub fn set_treasury_id(&mut self, treasury_id: impl Into<String>) {
        self.treasury_id = treasury_id.into();
    }

    pub fn set_signer(&mut self, signer: impl Signer + 'static) {
        self.signer = Some(Arc::new(signer));
    }

    pub fn get<T: DeserializeOwned>(&self, resource_name: &str) -> Result<T, ClientError> {
        let value = self.get_json(resource_name)?;
        Ok(serde_json::from_value(value)?)
    }

    pub fn get_json(&self, resource_name: &str) -> Result<Value, ClientError> {
        self.request_json("GET", &resource_path(resource_name), None::<&Value>, None)
    }

    pub fn list(
        &self,
        resource_type: ResourceType,
        opts: ListOptions,
    ) -> Result<ListResponse, ClientError> {
        let path = if let Some(parent_id) = opts.parent_id.as_deref() {
            let parent = resource_type.parent_path().ok_or_else(|| {
                ClientError::Message(format!("{} is not a nested resource", resource_type))
            })?;
            format!("/{parent}/{parent_id}/{}", resource_type.path())
        } else {
            format!("/{}", resource_type.path())
        };

        let mut url = self.api_url(&path)?;
        {
            let mut query = url.query_pairs_mut();
            if let Some(filter) = opts.filter.as_deref() {
                query.append_pair("filter", filter);
            }
            if let Some(order_by) = opts.order_by.as_deref() {
                query.append_pair("order_by", order_by);
            }
            if let Some(page_size) = opts.page_size {
                query.append_pair("page_size", &page_size.to_string());
            }
            if let Some(page_token) = opts.page_token.as_deref() {
                query.append_pair("page_token", page_token);
            }
            if opts.show_deleted {
                query.append_pair("show_deleted", "true");
            }
        }

        let body = self.send("GET", url, None::<&Value>, None)?;
        let mut resources = Vec::new();
        if let Some(arr) = body.get(resource_type.path()).and_then(Value::as_array) {
            resources.extend(arr.iter().cloned());
        } else if let Some(obj) = body.as_object() {
            for (key, value) in obj {
                if key != "next_page_token"
                    && key != "total_size"
                    && let Some(arr) = value.as_array()
                {
                    resources.extend(arr.iter().cloned());
                    break;
                }
            }
        }
        Ok(ListResponse {
            resources,
            next_page_token: body
                .get("next_page_token")
                .and_then(Value::as_str)
                .map(str::to_string),
            total_size: body.get("total_size").and_then(Value::as_u64),
        })
    }

    pub fn create<T: Serialize>(
        &self,
        resource_type: ResourceType,
        id: Option<&str>,
        parent_id: Option<&str>,
        body: &T,
    ) -> Result<String, ClientError> {
        let path = build_resource_path(resource_type, id, parent_id)?;
        self.extract_operation(self.request_json("POST", &path, Some(body), None)?)
    }

    pub fn update<T: Serialize>(
        &self,
        resource_name: &str,
        body: &T,
    ) -> Result<String, ClientError> {
        let current = self.get_json(resource_name)?;
        let update = serde_json::to_value(body)?;
        let merged = merge_update(current, update);
        self.extract_operation(self.request_json(
            "PUT",
            &resource_path(resource_name),
            Some(&merged),
            None,
        )?)
    }

    pub fn delete(&self, resource_name: &str) -> Result<String, ClientError> {
        self.extract_operation(self.request_json(
            "DELETE",
            &resource_path(resource_name),
            None::<&Value>,
            None,
        )?)
    }

    pub fn custom<T: Serialize>(
        &self,
        method: &str,
        path: &str,
        body: Option<&T>,
    ) -> Result<Value, ClientError> {
        self.request_json(method, path, body, None)
    }

    pub fn create_account(
        &self,
        id: Option<&str>,
        req: &CreateAccountRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Account, id, None, req)
    }

    pub fn create_address(
        &self,
        chain_id: &str,
        id: Option<&str>,
        req: &CreateAddressRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Address, id, Some(chain_id), req)
    }

    pub fn create_access_rule(
        &self,
        id: Option<&str>,
        req: &CreateRuleRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::AccessRule, id, None, req)
    }

    pub fn create_transfer_rule(
        &self,
        id: Option<&str>,
        req: &CreateRuleRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::TransferRule, id, None, req)
    }

    pub fn create_call_rule(
        &self,
        id: Option<&str>,
        req: &CreateRuleRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::CallRule, id, None, req)
    }

    pub fn create_staking_rule(
        &self,
        id: Option<&str>,
        req: &CreateRuleRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::StakingRule, id, None, req)
    }

    pub fn create_key(
        &self,
        id: Option<&str>,
        req: &CreateKeyRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Key, id, None, req)
    }

    pub fn create_credential(
        &self,
        user_id: &str,
        id: Option<&str>,
        req: &CreateCredentialRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Credential, id, Some(user_id), req)
    }

    pub fn create_user(
        &self,
        id: Option<&str>,
        req: &CreateUserRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::User, id, None, req)
    }

    pub fn create_asset(
        &self,
        chain_id: &str,
        id: Option<&str>,
        req: &CreateAssetRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Asset, id, Some(chain_id), req)
    }

    pub fn create_chain(
        &self,
        id: Option<&str>,
        req: &CreateChainRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Chain, id, None, req)
    }

    pub fn create_staking(
        &self,
        id: Option<&str>,
        req: &CreateStakingRequest,
    ) -> Result<String, ClientError> {
        self.create(ResourceType::Staking, id, None, req)
    }

    pub fn create_resource(
        &self,
        resource_type: ResourceType,
        id: Option<&str>,
        parent_id: Option<&str>,
        req: &CreateResourceRequest,
    ) -> Result<String, ClientError> {
        self.create(resource_type, id, parent_id, req)
    }

    fn request_json<T: Serialize>(
        &self,
        method: &str,
        path: &str,
        body: Option<&T>,
        tag: Option<&str>,
    ) -> Result<Value, ClientError> {
        let url = self.api_url(path)?;
        self.send(method, url, body, tag)
    }

    fn send<T: Serialize>(
        &self,
        method: &str,
        url: Url,
        body: Option<&T>,
        tag: Option<&str>,
    ) -> Result<Value, ClientError> {
        let request_body = if method == "GET" {
            Vec::new()
        } else if let Some(body) = body {
            serde_json::to_vec(body)?
        } else {
            b"{}".to_vec()
        };

        let mut headers = self.headers()?;
        if method != "GET" {
            headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        }

        if let Some(signer) = self.signer.as_deref() {
            let path_and_query = url[url::Position::BeforePath..].to_string();
            let signed = sign_request(
                signer,
                method,
                &path_and_query,
                &request_body,
                &self.treasury_id,
                self.host_id.as_deref(),
                tag,
            )?;
            headers.insert(
                "content-digest",
                HeaderValue::from_str(&signed.content_digest)?,
            );
            headers.insert(
                "signature-input",
                HeaderValue::from_str(&signed.signature_input)?,
            );
            headers.insert("signature", HeaderValue::from_str(&signed.signature)?);
        }

        let mut req = self
            .http
            .request(method.parse().unwrap(), url)
            .headers(headers);
        if method != "GET" {
            req = req.body(request_body);
        }
        let resp = req.send()?;
        let status_code = resp.status().as_u16();
        let text = resp.text()?;
        if status_code >= 400 {
            return Err(parse_api_error(status_code, &text));
        }
        if text.trim().is_empty() {
            Ok(json!({}))
        } else {
            Ok(serde_json::from_str(&text)?)
        }
    }

    fn api_url(&self, path: &str) -> Result<Url, ClientError> {
        let clean = path.trim_start_matches('/');
        Ok(self.base_url.join(&format!("v1/{clean}"))?)
    }

    fn headers(&self) -> Result<HeaderMap, ClientError> {
        let mut headers = HeaderMap::new();
        headers.insert(ACCEPT, HeaderValue::from_static("application/json"));
        headers.insert("Treasury", HeaderValue::from_str(&self.treasury_id)?);
        if let Some(host_id) = self.host_id.as_deref() {
            headers.insert("Treasury-Host", HeaderValue::from_str(host_id)?);
        }
        if let Some(api_key) = self.api_key.as_deref() {
            headers.insert(
                AUTHORIZATION,
                HeaderValue::from_str(&format!("Bearer {api_key}"))?,
            );
        }
        Ok(headers)
    }

    fn extract_operation(&self, value: Value) -> Result<String, ClientError> {
        for key in ["name", "operation", "operation_name"] {
            if let Some(name) = value.get(key).and_then(Value::as_str) {
                return Ok(name.to_string());
            }
        }
        if let Some(obj) = value.as_object() {
            for value in obj.values() {
                if let Some(name) = value.get("name").and_then(Value::as_str) {
                    return Ok(name.to_string());
                }
            }
        }
        Err(ClientError::Message(format!(
            "operation name missing from response: {value}"
        )))
    }
}

pub struct ClientBuilder {
    base_url: String,
    treasury_id: String,
    host_id: Option<String>,
    api_key: Option<String>,
    timeout: Duration,
    signer: Option<Arc<dyn Signer>>,
}

impl ClientBuilder {
    pub fn new(treasury_id: impl Into<String>) -> Self {
        Self {
            base_url: DEFAULT_BASE_URL.to_string(),
            treasury_id: treasury_id.into(),
            host_id: None,
            api_key: None,
            timeout: Duration::from_secs(30),
            signer: None,
        }
    }

    pub fn base_url(mut self, base_url: impl Into<String>) -> Self {
        self.base_url = base_url.into();
        self
    }

    pub fn host_id(mut self, host_id: impl Into<String>) -> Self {
        self.host_id = Some(host_id.into());
        self
    }

    pub fn api_key(mut self, api_key: impl Into<String>) -> Self {
        self.api_key = Some(normalize_api_key(&api_key.into()));
        self
    }

    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    pub fn signer(mut self, signer: impl Signer + 'static) -> Self {
        self.signer = Some(Arc::new(signer));
        self
    }

    pub fn build(self) -> Result<Client, ClientError> {
        if self.treasury_id.trim().is_empty() {
            return Err(ClientError::MissingTreasuryId);
        }
        let base_url = normalize_base_url(&self.base_url)?;
        if is_default_base_url(&base_url) && self.api_key.is_none() {
            return Err(ClientError::MissingApiKey(DEFAULT_BASE_URL));
        }
        let http = HttpClient::builder().timeout(self.timeout).build()?;
        Ok(Client {
            base_url,
            treasury_id: self.treasury_id,
            host_id: self.host_id,
            api_key: self.api_key,
            http,
            signer: self.signer,
        })
    }
}

pub fn lookup_treasury_id(
    base_url: impl AsRef<str>,
    api_key: Option<&str>,
) -> Result<String, ClientError> {
    let base_url = normalize_base_url(base_url.as_ref())?;
    let http = HttpClient::builder()
        .timeout(Duration::from_secs(30))
        .build()?;
    for path in ["/v1/treasury", "/v1/treasuries?page_size=1"] {
        let mut headers = HeaderMap::new();
        headers.insert(ACCEPT, HeaderValue::from_static("application/json"));
        if let Some(api_key) = api_key {
            headers.insert(
                AUTHORIZATION,
                HeaderValue::from_str(&format!("Bearer {}", normalize_api_key(api_key)))?,
            );
        }
        let url = base_url.join(path.trim_start_matches('/'))?;
        let resp = http.get(url).headers(headers).send()?;
        if !resp.status().is_success() {
            continue;
        }
        let value: Value = resp.json()?;
        if let Some(id) = extract_treasury_id(&value) {
            return Ok(id);
        }
    }
    Err(ClientError::Message(format!(
        "could not discover treasury id from {base_url}"
    )))
}

pub fn build_resource_path(
    resource_type: ResourceType,
    id: Option<&str>,
    parent_id: Option<&str>,
) -> Result<String, ClientError> {
    let mut path = String::new();
    if let Some(parent) = resource_type.parent_path() {
        let parent_id = parent_id.ok_or_else(|| {
            ClientError::Message(format!("{resource_type} requires parent id under {parent}"))
        })?;
        path.push('/');
        path.push_str(parent);
        path.push('/');
        path.push_str(parent_id.trim_matches('/'));
    }
    path.push('/');
    path.push_str(resource_type.path());
    if let Some(id) = id.filter(|id| !id.is_empty()) {
        path.push('/');
        path.push_str(id.trim_matches('/'));
    }
    Ok(path)
}

pub fn normalize_api_key(api_key: &str) -> String {
    let trimmed = api_key.trim();
    if trimmed.is_empty() {
        return String::new();
    }
    if BASE64.decode(trimmed).is_ok() {
        trimmed.to_string()
    } else {
        BASE64.encode(trimmed)
    }
}

fn normalize_base_url(base_url: &str) -> Result<Url, ClientError> {
    let mut base = base_url.trim().trim_end_matches('/').to_string();
    if base.is_empty() {
        base = DEFAULT_BASE_URL.trim_end_matches('/').to_string();
    }
    base.push('/');
    Ok(Url::parse(&base)?)
}

fn is_default_base_url(base_url: &Url) -> bool {
    base_url.as_str().trim_end_matches('/') == DEFAULT_BASE_URL.trim_end_matches('/')
}

fn resource_path(resource_name: &str) -> String {
    format!(
        "/{}",
        resource_name
            .trim_start_matches("/v1/")
            .trim_start_matches('/')
    )
}

fn merge_update(current: Value, update: Value) -> Value {
    match (current, update) {
        (Value::Object(mut current), Value::Object(update)) => {
            for (key, value) in update {
                current.insert(key, value);
            }
            Value::Object(current)
        }
        (_, update) => update,
    }
}

fn parse_api_error(status_code: u16, text: &str) -> ClientError {
    match serde_json::from_str::<ErrorResponse>(text) {
        Ok(err) => ClientError::Api {
            status_code,
            code: err.code,
            status: err.status,
            message: err.message,
            details: err.details,
        },
        Err(_) => ClientError::Api {
            status_code,
            code: json!("UNKNOWN"),
            status: "Unknown".to_string(),
            message: text.to_string(),
            details: Vec::new(),
        },
    }
}

fn extract_treasury_id(value: &Value) -> Option<String> {
    if let Some(name) = value.get("name").and_then(Value::as_str) {
        return Some(name.trim_start_matches("treasuries/").to_string());
    }
    for (key, value) in value.as_object()? {
        if key == "next_page_token" || key == "total_size" {
            continue;
        }
        let item = value.as_array()?.first()?;
        if let Some(name) = item.get("name").and_then(Value::as_str) {
            return Some(name.trim_start_matches("treasuries/").to_string());
        }
    }
    None
}
