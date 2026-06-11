//! Cordial Treasury Rust SDK.
//!
//! This crate provides a typed client facade, keyring/signing helpers, and a
//! compact CSL runner for Treasury scripts.

pub mod client;
pub mod csl;
pub mod keyring;
pub mod signing;
pub mod types;

pub use client::{
    Client, ClientBuilder, ClientError, ListOptions, ListResponse, lookup_treasury_id,
};
pub use keyring::Keyring;
pub use signing::{Identity, Signer, SigningAlgorithm};
pub use types::*;
