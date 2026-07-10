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
    /// Query indexed MindWeaver data.
    Query {
        #[command(subcommand)]
        command: QueryCommand,
    },
    /// Todo dashboard workflows.
    Todos {
        #[command(subcommand)]
        command: TodosCommand,
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

#[derive(Debug, Subcommand)]
enum QueryCommand {
    /// Query notes.
    Notes {
        /// Fetch a single note by DB id.
        #[arg(long)]
        id: Option<i64>,
        /// Fetch a single note by registered uid.
        #[arg(long)]
        uid: Option<String>,
        /// Output format: json|text.
        #[arg(long, default_value = "json")]
        format: String,
        /// Search notes by title substring.
        #[arg(long)]
        search: Option<String>,
        /// Comma-separated tags.
        #[arg(long)]
        tags: Option<String>,
        /// Filter notes by metadata domain.
        #[arg(long)]
        domain: Option<String>,
        /// Filter glossary notes by category folder name.
        #[arg(long)]
        category: Option<String>,
        /// Max results when listing.
        #[arg(long, default_value_t = 50)]
        limit: i64,
        /// Offset when listing.
        #[arg(long, default_value_t = 0)]
        offset: i64,
    },
    /// List all note domains.
    Domains,
    /// Query active task-index todos from markdown notes.
    Todos {
        /// Output format: json|text.
        #[arg(long, default_value = "json")]
        format: String,
    },
}

#[derive(Debug, Subcommand)]
enum TodosCommand {
    /// Sync task-index todos into the dashboard, applying dashboard checkbox changes back to source notes first.
    Sync,
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
        Command::Query { command } => query_command(command),
        Command::Todos { command } => todos_command(command),
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

fn todos_command(command: TodosCommand) -> Result<()> {
    match command {
        TodosCommand::Sync => todos_sync_command(),
    }
}

fn todos_sync_command() -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    let stats = mw_notes::sync_dashboard_from_task_index_notes(root, &cfg.dashboard_path)
        .context("sync dashboard from task-index notes")?;
    println!(
        "✅ synced {} task(s) from {} active task-index note(s) ({} markdown note(s) scanned)",
        stats.synced_tasks, stats.active_task_index_notes, stats.scanned_markdown_notes
    );
    for group in [
        "Code",
        "Action",
        "Reading",
        "Amusement",
        "Music",
        "Exercise",
        "Love",
    ] {
        if let Some(n) = stats.tasks_by_area.get(group).filter(|n| **n > 0) {
            println!("  • {group}: {n}");
        }
    }
    if stats.source_writebacks > 0 {
        println!(
            "↩️ applied {} completion update(s) back to {} source note(s)",
            stats.source_writebacks, stats.source_files_updated
        );
    }
    println!("📝 dashboard updated: {}", cfg.dashboard_path);
    Ok(())
}

fn query_command(command: QueryCommand) -> Result<()> {
    match command {
        QueryCommand::Notes {
            id,
            uid,
            format,
            search,
            tags,
            domain,
            category,
            limit,
            offset,
        } => query_notes_command(QueryNotesOptions {
            id,
            uid,
            format,
            search,
            tags,
            domain,
            category,
            limit,
            offset,
        }),
        QueryCommand::Domains => query_domains_command(),
        QueryCommand::Todos { format } => query_todos_command(&format),
    }
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct TodoResult {
    id: String,
    #[serde(rename = "noteID")]
    note_id: String,
    note_title: String,
    path: String,
    #[serde(rename = "sourceID")]
    source_id: String,
    task_scope: String,
    todo_section: String,
    title: String,
    is_done: bool,
    status: String,
    raw_status: String,
    section: String,
    #[serde(rename = "taskGroupID")]
    task_group_id: String,
    depth: i32,
    line_number: usize,
    metadata: mw_notes::TodoMetadata,
    area: String,
    priority: String,
    energy: String,
    weight_override: String,
    due: String,
    start: String,
    estimate: String,
    effective_weight: f64,
}

fn query_todos_command(format: &str) -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    let (todos, _) =
        mw_notes::list_active_task_index_todos(root).context("list active task-index todos")?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    let mut results = Vec::with_capacity(todos.len());

    for todo in todos {
        let metadata = todo.metadata.clone();
        let mut status = metadata.status.trim().to_string();
        if status.is_empty() {
            status = todo.todo_section.clone();
        }
        if status.trim().is_empty() {
            status = "Inbox".to_string();
        }
        let raw_status = if todo.done {
            status = "Done".to_string();
            "x".to_string()
        } else {
            " ".to_string()
        };
        let note_id = db
            .note_id_by_path(&todo.source_path)?
            .map(|id| id.to_string())
            .unwrap_or_default();

        results.push(TodoResult {
            id: todo.id,
            note_id,
            note_title: todo.note_title,
            path: todo.source_path.clone(),
            source_id: todo.source_id,
            task_scope: todo.task_scope,
            todo_section: todo.todo_section.clone(),
            title: todo.text,
            is_done: todo.done,
            status,
            raw_status,
            section: todo.area.clone(),
            task_group_id: todo.area,
            depth: 0,
            line_number: todo.line,
            area: metadata.area.clone(),
            priority: metadata.priority.clone(),
            energy: metadata.energy.clone(),
            weight_override: metadata.weight_override.clone(),
            due: metadata.due.clone(),
            start: metadata.start.clone(),
            estimate: metadata.estimate.clone(),
            effective_weight: metadata.effective_weight,
            metadata,
        });
    }

    if format.trim().eq_ignore_ascii_case("text") {
        for todo in results {
            let checkbox = if todo.is_done { "x" } else { " " };
            println!(
                "- [{checkbox}] {} ({}, {}, {}, {})",
                todo.title, todo.area, todo.status, todo.priority, todo.path
            );
        }
        return Ok(());
    }
    write_json(&results)
}

fn query_domains_command() -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    write_json(&db.list_domains()?)
}

struct QueryNotesOptions {
    id: Option<i64>,
    uid: Option<String>,
    format: String,
    search: Option<String>,
    tags: Option<String>,
    domain: Option<String>,
    category: Option<String>,
    limit: i64,
    offset: i64,
}

fn query_notes_command(opts: QueryNotesOptions) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;

    if let Some(search) = trim_non_empty(opts.search) {
        return write_json(&db.search_notes_by_title(&search)?);
    }

    if let Some(tags) = trim_non_empty(opts.tags) {
        return write_json(&db.list_notes_by_tags(&split_csv(&tags))?);
    }

    let mut domain = trim_non_empty(opts.domain);
    let category = trim_non_empty(opts.category);
    if category.is_some() {
        if domain.is_none() {
            domain = Some("glossary".to_string());
        }
        if !domain
            .as_deref()
            .is_some_and(|d| d.eq_ignore_ascii_case("glossary"))
        {
            bail!("--category is only supported for domain=glossary");
        }
    }

    if let Some(domain) = domain {
        let mut notes = db.list_notes_by_domain(&domain)?;
        if let Some(category) = category {
            notes = filter_glossary_notes_by_category(notes, &category);
        }
        return write_json(&paginate_notes(notes, opts.limit, opts.offset));
    }

    if let Some(uid) = trim_non_empty(opts.uid) {
        let Some(note) = db.get_note_by_uid(&uid)? else {
            bail!("No note registered with uid: {uid}");
        };
        return write_note(&note, &opts.format);
    }

    if let Some(id) = opts.id.filter(|id| *id != 0) {
        let Some(note) = db.get_note_by_id(id)? else {
            bail!("No note found with id: {id}");
        };
        return write_note(&note, &opts.format);
    }

    write_json(&db.list_notes(opts.limit, opts.offset)?)
}

fn write_note(note: &mw_db::NoteRecord, format: &str) -> Result<()> {
    if format.trim().eq_ignore_ascii_case("text") {
        println!("{}", note.title);
        if !note.uid.is_empty() {
            println!("uid: {}", note.uid);
        }
        println!("path: {}", note.path);
        println!();
        print!("{}", note.content);
        return Ok(());
    }
    write_json(note)
}

fn write_json(value: &impl serde::Serialize) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(value)?);
    Ok(())
}

fn trim_non_empty(value: Option<String>) -> Option<String> {
    value
        .map(|v| v.trim().to_string())
        .filter(|v| !v.is_empty())
}

fn split_csv(value: &str) -> Vec<String> {
    value
        .split(',')
        .map(|part| part.trim().to_string())
        .filter(|part| !part.is_empty())
        .collect()
}

fn filter_glossary_notes_by_category(
    notes: Vec<mw_db::NoteRecord>,
    wanted_category: &str,
) -> Vec<mw_db::NoteRecord> {
    let wanted_category = wanted_category.trim();
    if wanted_category.is_empty() {
        return notes;
    }
    notes
        .into_iter()
        .filter(|note| {
            glossary_category_from_path(&note.path)
                .is_some_and(|category| category.eq_ignore_ascii_case(wanted_category))
        })
        .collect()
}

fn glossary_category_from_path(note_path: &str) -> Option<String> {
    let normalized = note_path.trim().replace('\\', "/");
    let parent = normalized.rsplit_once('/')?.0;
    if parent.is_empty() || parent == "." || parent == "/" {
        return None;
    }
    let category = parent.rsplit('/').next()?.trim();
    if category.is_empty() || category == "." || category == "/" {
        None
    } else {
        Some(category.to_string())
    }
}

fn paginate_notes(
    notes: Vec<mw_db::NoteRecord>,
    limit: i64,
    offset: i64,
) -> Vec<mw_db::NoteRecord> {
    let offset = offset.max(0) as usize;
    if offset >= notes.len() {
        return Vec::new();
    }
    let limit = if limit <= 0 {
        notes.len()
    } else {
        limit as usize
    };
    let end = (offset + limit).min(notes.len());
    notes.into_iter().skip(offset).take(end - offset).collect()
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
