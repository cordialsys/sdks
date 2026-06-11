use serde_yaml::Value;
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process;

fn main() {
    let args = Args::parse();
    if let Err(err) = generate(&args.spec_path, &args.output_path) {
        eprintln!("error: {err}");
        process::exit(1);
    }
}

struct Args {
    spec_path: PathBuf,
    output_path: PathBuf,
}

impl Args {
    fn parse() -> Self {
        let mut args = env::args_os().skip(1);
        let spec_path = args
            .next()
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from("../openapi/treasury.yaml"));
        let output_path = args
            .next()
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from("src/types/openapi.rs"));

        if args.next().is_some() {
            eprintln!("usage: cargo run --manifest-path typegen/Cargo.toml -- [spec] [output]");
            process::exit(2);
        }

        Self {
            spec_path,
            output_path,
        }
    }
}

fn generate(spec_path: &Path, output_path: &Path) -> Result<(), String> {
    let spec = fs::read_to_string(spec_path)
        .map_err(|err| format!("read OpenAPI spec {}: {err}", spec_path.display()))?;
    let spec: Value = serde_yaml::from_str(&spec)
        .map_err(|err| format!("parse OpenAPI spec {}: {err}", spec_path.display()))?;
    let schemas = spec
        .get("components")
        .and_then(|v| v.get("schemas"))
        .and_then(Value::as_mapping)
        .ok_or_else(|| "missing components.schemas".to_string())?;

    let mut enums = BTreeMap::new();
    for (name, schema) in schemas {
        let Some(name) = name.as_str() else {
            continue;
        };
        collect_enum(name, schema, &mut enums);
    }

    let mut out = format!(
        "// Generated from {} by rust/typegen. Do not edit by hand.\n",
        spec_path.display()
    );
    out.push_str("use serde::{Deserialize, Serialize};\n\n");
    for (name, generated) in enums {
        out.push_str(&generated.render(&name));
        out.push('\n');
    }

    if let Some(parent) = output_path.parent()
        && !parent.as_os_str().is_empty()
    {
        fs::create_dir_all(parent)
            .map_err(|err| format!("create output directory {}: {err}", parent.display()))?;
    }

    fs::write(output_path, out)
        .map_err(|err| format!("write generated types {}: {err}", output_path.display()))?;
    println!("wrote {}", output_path.display());
    Ok(())
}

fn collect_enum(name: &str, schema: &Value, enums: &mut BTreeMap<String, GeneratedEnum>) {
    if let Some(values) = schema.get("enum").and_then(Value::as_sequence) {
        let description = schema
            .get("description")
            .and_then(Value::as_str)
            .map(str::to_string);
        let values = values
            .iter()
            .filter_map(Value::as_str)
            .map(str::to_string)
            .collect::<Vec<_>>();
        if !values.is_empty() {
            enums.insert(
                name.to_string(),
                GeneratedEnum {
                    description,
                    values,
                },
            );
        }
        return;
    }

    if let Some(mapping) = schema.as_mapping() {
        for (key, value) in mapping {
            if key
                .as_str()
                .is_some_and(|k| k == "properties" || k == "$defs")
                && let Some(properties) = value.as_mapping()
            {
                for (prop_name, prop_schema) in properties {
                    let Some(prop_name) = prop_name.as_str() else {
                        continue;
                    };
                    let type_name = format!("{name}{}", to_pascal(prop_name));
                    collect_enum(&type_name, prop_schema, enums);
                }
            }
        }
    }
}

struct GeneratedEnum {
    description: Option<String>,
    values: Vec<String>,
}

impl GeneratedEnum {
    fn render(&self, name: &str) -> String {
        let mut out = String::new();
        if let Some(description) = self.description.as_deref() {
            for line in description.lines() {
                let line = line.trim();
                if line.is_empty() {
                    out.push_str("///\n");
                } else {
                    out.push_str("/// ");
                    out.push_str(line);
                    out.push('\n');
                }
            }
        }
        out.push_str("#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]\n");
        out.push_str(&format!("pub enum {} {{\n", to_pascal(name)));
        for value in &self.values {
            out.push_str(&format!("    #[serde(rename = {:?})]\n", value));
            out.push_str(&format!("    {},\n", to_pascal(value)));
        }
        out.push_str("}\n");
        out
    }
}

fn to_pascal(input: &str) -> String {
    let mut out = String::new();
    let mut upper = true;
    for ch in input.chars() {
        if ch.is_ascii_alphanumeric() {
            if upper {
                out.extend(ch.to_uppercase());
                upper = false;
            } else {
                out.push(ch);
            }
        } else {
            upper = true;
        }
    }
    if out.chars().next().is_some_and(|ch| ch.is_ascii_digit()) {
        out.insert(0, 'V');
    }
    if out.is_empty() {
        "Unknown".to_string()
    } else {
        out
    }
}
