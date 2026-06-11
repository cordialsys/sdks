use serde_yaml::Value;
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::path::PathBuf;

fn main() {
    println!("cargo:rerun-if-changed=../openapi/treasury.yaml");
    let spec = fs::read_to_string("../openapi/treasury.yaml").expect("read OpenAPI spec");
    let spec: Value = serde_yaml::from_str(&spec).expect("parse OpenAPI spec");
    let schemas = spec
        .get("components")
        .and_then(|v| v.get("schemas"))
        .and_then(Value::as_mapping)
        .expect("components.schemas");

    let mut enums = BTreeMap::new();
    for (name, schema) in schemas {
        let Some(name) = name.as_str() else {
            continue;
        };
        collect_enum(name, schema, &mut enums);
    }

    let mut out = String::from("// Generated from ../openapi/treasury.yaml by build.rs.\n");
    out.push_str("use serde::{Deserialize, Serialize};\n\n");
    for (name, generated) in enums {
        out.push_str(&generated.render(&name));
        out.push('\n');
    }

    let out_file = PathBuf::from(env::var("OUT_DIR").expect("OUT_DIR")).join("openapi_types.rs");
    println!("generating {}", out_file.display());
    fs::write(out_file, out).expect("write generated OpenAPI types");
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
                out.push_str("/// ");
                out.push_str(line.trim());
                out.push('\n');
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
