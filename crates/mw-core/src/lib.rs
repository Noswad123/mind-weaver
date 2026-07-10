pub const APP_NAME: &str = "mw";
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
pub const APP_DIR_NAME: &str = "mind-weaver";
pub const EMBEDDED_SCHEMA_PATH: &str = "embedded://mind-weaver/schema.sql";
pub const EMBEDDED_COMMAND_SCHEMA_PATH: &str = "embedded://mind-weaver/command-schema.sql";

pub mod config;

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
pub struct AppPaths {
    pub notes_dir: Option<String>,
    pub config_path: Option<String>,
    pub data_dir: Option<String>,
}

impl AppPaths {
    pub fn empty() -> Self {
        Self {
            notes_dir: None,
            config_path: None,
            data_dir: None,
        }
    }
}
