use std::{collections::BTreeSet, fs, path::Path};

use anyhow::{Context, Result, bail};
use clap::{Parser, Subcommand};
use mw_core::config::{self, CheckStatus, InitOptions};
use mw_notes::ParseContext;

#[derive(Debug, Parser)]
#[command(name = "mw")]
#[command(about = "MindWeaver Rust port", version)]
struct Cli {
    #[command(subcommand)]
    command: Option<Command>,
}

#[derive(Debug, Subcommand)]
enum Command {
    /// Create a MindWeaver config and local directories.
    Init {
        /// Config file path.
        #[arg(long)]
        config: Option<String>,
        /// Root directory for markdown notes.
        #[arg(long)]
        notes_dir: Option<String>,
        /// SQLite database path.
        #[arg(long)]
        db_path: Option<String>,
        /// Inbox note path.
        #[arg(long)]
        inbox: Option<String>,
        /// Overwrite an existing config file.
        #[arg(long)]
        force: bool,
    },
    /// Check MindWeaver configuration and local dependencies.
    Doctor,
    /// Inspect MindWeaver configuration.
    Config {
        #[command(subcommand)]
        command: ConfigCommand,
    },
    /// Manage local SQLite databases.
    Db {
        #[command(subcommand)]
        command: DbCommand,
    },
    /// Notes workflows.
    Notes {
        #[command(subcommand)]
        command: NotesCommand,
    },
    /// Launch the ratatui workspace shell.
    Tui,
    /// Print Rust port version information.
    Version,
    /// Show how to run the legacy Go implementation while porting.
    Legacy {
        /// Arguments intended for the legacy Go mw command.
        #[arg(trailing_var_arg = true)]
        args: Vec<String>,
    },
}

#[derive(Debug, Subcommand)]
enum ConfigCommand {
    /// Print the active config path.
    Path,
    /// Print merged configuration.
    Show,
}

#[derive(Debug, Subcommand)]
enum DbCommand {
    /// Initialize notes and commands SQLite databases from configured schemas.
    Init,
    /// Open configured databases and verify core tables exist.
    Check,
}

#[derive(Debug, Subcommand)]
enum NotesCommand {
    /// Ingest markdown notes into SQLite.
    Ingest,
    /// Register note IDs and detect registry conflicts.
    Register,
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command.unwrap_or(Command::Tui) {
        Command::Init {
            config,
            notes_dir,
            db_path,
            inbox,
            force,
        } => init_command(config, notes_dir, db_path, inbox, force),
        Command::Doctor => doctor_command(),
        Command::Config { command } => config_command(command),
        Command::Db { command } => db_command(command),
        Command::Notes { command } => notes_command(command),
        Command::Tui => mw_tui::run(),
        Command::Version => {
            println!("{} {}", mw_core::APP_NAME, mw_core::VERSION);
            Ok(())
        }
        Command::Legacy { args } => {
            let suffix = if args.is_empty() {
                String::new()
            } else {
                format!(" {}", args.join(" "))
            };
            println!("Legacy Go fallback:");
            println!("  cd legacy/go && go run ./cmd/mw --{suffix}");
            Ok(())
        }
    }
}

fn notes_command(command: NotesCommand) -> Result<()> {
    match command {
        NotesCommand::Ingest => ingest_notes_command(),
        NotesCommand::Register => register_notes_command(),
    }
}

fn register_notes_command() -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    let mut db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    let result = mw_notes::build_registry(root).context("build registry")?;

    let mut entries = Vec::new();
    let mut conflicts = Vec::new();

    for rel in &result.missing_hub {
        conflicts.push(mw_db::RegistryConflict {
            note_id: db.note_id_by_path(rel)?,
            uid: None,
            path: rel.clone(),
            is_hub: true,
            reason: "MISSING_HUB_ID".to_string(),
        });
    }

    for duplicate in &result.duplicates {
        for rel in &duplicate.paths {
            conflicts.push(mw_db::RegistryConflict {
                note_id: db.note_id_by_path(rel)?,
                uid: Some(duplicate.id.clone()),
                path: rel.clone(),
                is_hub: mw_notes::is_hub_note_path(rel),
                reason: "DUPLICATE_ID".to_string(),
            });
        }
    }

    for entry in result.registry.entries.values() {
        let note_id = db.note_id_by_path(&entry.path)?;
        let is_hub = mw_notes::is_hub_note_path(&entry.path);
        if note_id.is_none() {
            conflicts.push(mw_db::RegistryConflict {
                note_id: None,
                uid: Some(entry.id.clone()),
                path: entry.path.clone(),
                is_hub,
                reason: "NOTE_NOT_IN_DB".to_string(),
            });
            continue;
        }
        entries.push(mw_db::RegisteredNoteId {
            note_id,
            uid: entry.id.clone(),
            path: entry.path.clone(),
            is_hub,
        });
    }

    db.replace_registry(&entries, &conflicts)?;
    println!(
        "✅ Registry updated: {} registered, {} conflict(s)",
        entries.len(),
        conflicts.len()
    );
    Ok(())
}

fn ingest_notes_command() -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    let mut db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    let markdown_files = collect_markdown_files(root)?;
    let mut disk_paths = BTreeSet::new();
    let mut count = 0usize;

    for path in markdown_files {
        let rel = relative_slash_path(root, &path)?;
        if rel.starts_with(".git/") {
            continue;
        }
        disk_paths.insert(rel.clone());

        let content = fs::read_to_string(&path).with_context(|| format!("read {rel}"))?;
        let parsed = mw_notes::parse_note_with_context(
            &content,
            &ParseContext {
                source_rel_path: rel.clone(),
                notes_root_abs: cfg.notes_dir.clone(),
            },
        );
        db.upsert_parsed_note(&rel, &parsed)
            .with_context(|| format!("upsert {rel}"))?;
        count += 1;
    }

    let mut pruned = 0usize;
    for path in db.all_note_paths()? {
        if !disk_paths.contains(&path) {
            db.delete_note_by_path(&path)
                .with_context(|| format!("prune {path}"))?;
            pruned += 1;
        }
    }

    println!("✅ Pruned {pruned} notes");
    println!("✅ ingested {count} notes");
    Ok(())
}

fn collect_markdown_files(root: &Path) -> Result<Vec<std::path::PathBuf>> {
    let mut out = Vec::new();
    collect_markdown_files_into(root, &mut out)?;
    out.sort();
    Ok(out)
}

fn collect_markdown_files_into(dir: &Path, out: &mut Vec<std::path::PathBuf>) -> Result<()> {
    for entry in fs::read_dir(dir).with_context(|| format!("read directory {}", dir.display()))? {
        let entry = entry?;
        let path = entry.path();
        let file_type = entry.file_type()?;
        if file_type.is_dir() {
            if entry.file_name() == ".git" {
                continue;
            }
            collect_markdown_files_into(&path, out)?;
            continue;
        }
        if file_type.is_file()
            && path
                .extension()
                .and_then(|ext| ext.to_str())
                .is_some_and(|ext| ext.eq_ignore_ascii_case("md"))
        {
            out.push(path);
        }
    }
    Ok(())
}

fn relative_slash_path(root: &Path, path: &Path) -> Result<String> {
    let rel = path
        .strip_prefix(root)
        .with_context(|| format!("make {} relative to {}", path.display(), root.display()))?;
    Ok(rel.to_string_lossy().replace('\\', "/"))
}

fn db_command(command: DbCommand) -> Result<()> {
    let cfg = config::load()?;
    match command {
        DbCommand::Init => {
            let report = mw_db::initialize_from_config(&cfg)?;
            println!("✅ SQLite databases initialized");
            println!("notes:    {}", report.notes_db_path);
            println!("commands: {}", report.commands_db_path);
        }
        DbCommand::Check => {
            mw_db::validate_from_config(&cfg)?;
            println!("✅ SQLite databases are ready");
            println!("notes:    {}", cfg.db_path);
            println!("commands: {}", cfg.commands_db_path);
        }
    }
    Ok(())
}

fn init_command(
    config_path: Option<String>,
    notes_dir: Option<String>,
    db_path: Option<String>,
    inbox_path: Option<String>,
    force: bool,
) -> Result<()> {
    let cfg = config::init(InitOptions {
        config_path,
        notes_dir,
        db_path,
        inbox_path,
        force,
    })?;

    println!("✅ MindWeaver initialized");
    println!("config: {}", cfg.config_path);
    println!("notes:  {}", cfg.notes_dir);
    println!("db:     {}", cfg.db_path);
    println!();
    println!("Next steps:");
    println!("  1. Add markdown notes under {}", cfg.notes_dir);
    println!("  2. Run `mw doctor`");
    println!("  3. Run `mw notes ingest` when ready");
    Ok(())
}

fn config_command(command: ConfigCommand) -> Result<()> {
    match command {
        ConfigCommand::Path => {
            println!("{}", config::active_config_path());
            Ok(())
        }
        ConfigCommand::Show => {
            let cfg = config::load()?;
            print!("{}", config::format_config(&cfg));
            Ok(())
        }
    }
}

fn doctor_command() -> Result<()> {
    let loaded = config::load();
    let (cfg, load_err) = match loaded {
        Ok(cfg) => (cfg, None),
        Err(err) => {
            let mut cfg = config::Config::default();
            cfg.config_path = config::active_config_path();
            (cfg, Some(err))
        }
    };

    let checks = config::doctor(cfg, load_err.as_ref());
    let mut failed = false;
    for check in checks {
        if check.status == CheckStatus::Fail {
            failed = true;
        }
        println!(
            "{} {:<20} {}",
            status_label(check.status),
            format!("{}:", check.name),
            check.message
        );
    }
    if failed {
        bail!("MindWeaver doctor found blocking issues");
    }
    Ok(())
}

fn status_label(status: CheckStatus) -> &'static str {
    match status {
        CheckStatus::Ok => "✅",
        CheckStatus::Warn => "⚠️ ",
        CheckStatus::Fail => "❌",
    }
}
