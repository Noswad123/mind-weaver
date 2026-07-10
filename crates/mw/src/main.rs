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
    /// Shortcut for `mw notes get`.
    #[command(alias = "summon")]
    Get {
        /// Fetch by DB id.
        #[arg(long)]
        id: Option<i64>,
        /// Search notes by title substring.
        #[arg(long)]
        search: Option<String>,
        /// Comma-separated tags.
        #[arg(long)]
        tags: Option<String>,
    },
    /// Shortcut for `mw notes sync`; `mw sync` is unsupported.
    Seal,
    /// Shortcut for `mw notes ingest`.
    #[command(alias = "banish")]
    Ingest {
        /// Prune notes from the DB that are no longer on disk.
        #[arg(long)]
        prune: bool,
    },
    /// Shortcut for `mw notes format`.
    #[command(alias = "meld")]
    Format {
        /// Format all markdown notes, not just hub notes.
        #[arg(long)]
        all: bool,
    },
    /// Shortcut for `mw notes register`.
    Register,
    /// Shortcut for `mw notes validate`.
    Validate {
        /// Validate all notes (currently default behavior).
        #[arg(long)]
        all: bool,
        /// Validate notes in a domain.
        #[arg(long)]
        domain: Option<String>,
    },
    /// Shortcut for `mw notes validate-registry`.
    ValidateRegistry,
    /// Shortcut for `mw notes graph`.
    #[command(alias = "loom")]
    Graph {
        /// Seed graph from matching note title/path/tag/domain text.
        #[arg(long)]
        search: Option<String>,
        /// Seed graph from notes in a domain.
        #[arg(long)]
        domain: Option<String>,
        /// Neighbor expansion depth from matched seed nodes.
        #[arg(long, default_value_t = 1)]
        depth: i64,
        /// Maximum nodes returned.
        #[arg(long, default_value_t = 250)]
        limit: i64,
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
    Tui {
        #[command(subcommand)]
        command: Option<TuiCommand>,
    },
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
    /// Run the notes pipeline: format, ingest, register, validate registry.
    Sync,
    /// Retrieve notes by id, title search, tags, or list recent notes.
    #[command(alias = "summon")]
    Get {
        /// Fetch by DB id.
        #[arg(long)]
        id: Option<i64>,
        /// Search notes by title substring.
        #[arg(long)]
        search: Option<String>,
        /// Comma-separated tags.
        #[arg(long)]
        tags: Option<String>,
    },
    /// Format hub notes and optionally normalize all markdown note ids/headings.
    Format {
        /// Format all markdown notes, not just hub notes.
        #[arg(long)]
        all: bool,
    },
    /// Ingest markdown notes into SQLite.
    Ingest {
        /// Prune notes from the DB that are no longer on disk.
        #[arg(long)]
        prune: bool,
    },
    /// Register note IDs and detect registry conflicts.
    Register,
    /// Validate note files on disk.
    Validate {
        /// Validate all notes (currently default behavior).
        #[arg(long)]
        all: bool,
        /// Validate notes in a domain.
        #[arg(long)]
        domain: Option<String>,
    },
    /// Validate DB-backed registry conflicts.
    ValidateRegistry,
    /// Launch the visual graph browser.
    Graph {
        /// Seed graph from matching note title/path/tag/domain text.
        #[arg(long)]
        search: Option<String>,
        /// Seed graph from notes in a domain.
        #[arg(long)]
        domain: Option<String>,
        /// Neighbor expansion depth from matched seed nodes.
        #[arg(long, default_value_t = 1)]
        depth: i64,
        /// Maximum nodes returned.
        #[arg(long, default_value_t = 250)]
        limit: i64,
    },
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
    /// Query note graph nodes and links.
    Graph {
        /// Seed graph from matching note title/path/tag/domain text.
        #[arg(long)]
        search: Option<String>,
        /// Seed graph from notes in a domain.
        #[arg(long)]
        domain: Option<String>,
        /// Neighbor expansion depth from matched seed nodes.
        #[arg(long, default_value_t = 1)]
        depth: i64,
        /// Maximum nodes returned.
        #[arg(long, default_value_t = 250)]
        limit: i64,
    },
    /// List recipe projections.
    Recipes {
        /// Scope projection to notes containing all listed domains; may be repeated or comma-separated.
        #[arg(long = "scope", alias = "scopes")]
        scope: Vec<String>,
    },
    /// Query a projection by structural domain.
    Projection {
        /// Projection name, e.g. recipe.
        projection: String,
        /// Scope projection to notes containing all listed domains; may be repeated or comma-separated.
        #[arg(long = "scope", alias = "scopes")]
        scope: Vec<String>,
    },
    /// List ingredients or recipe ingredient mentions.
    Ingredients {
        /// List ingredient mentions instead of canonical ingredients.
        #[arg(long)]
        mentions: bool,
        /// List unresolved mentions instead of canonical ingredients.
        #[arg(long)]
        unresolved: bool,
    },
    /// Query note ID registry and conflicts.
    Registry,
}

#[derive(Debug, Subcommand)]
enum TodosCommand {
    /// Sync task-index todos into the dashboard, applying dashboard checkbox changes back to source notes first.
    Sync,
    /// Toggle a task-index todo by query id and refresh dashboard.
    Toggle {
        /// Todo id returned by query todos.
        #[arg(long)]
        id: String,
    },
    /// Inspect a source-backed task-index todo as JSON.
    Inspect {
        /// Todo id returned by query todos.
        #[arg(long)]
        id: String,
    },
    /// Update task-index todo text or metadata and refresh dashboard.
    Update {
        /// Todo id returned by query todos; repeat for bulk edits.
        #[arg(long, required = true)]
        id: Vec<String>,
        /// Replace task text; single id only.
        #[arg(long)]
        title: Option<String>,
        /// Set todo area.
        #[arg(long)]
        area: Option<String>,
        /// Set priority p1..p5.
        #[arg(long)]
        priority: Option<String>,
        /// Set energy xsm|s|m|l|xl.
        #[arg(long)]
        energy: Option<String>,
        /// Set explicit weight override.
        #[arg(long)]
        weight: Option<String>,
        /// Set due date YYYY-MM-DD.
        #[arg(long)]
        due: Option<String>,
        /// Set start date YYYY-MM-DD.
        #[arg(long)]
        start: Option<String>,
        /// Set estimate minutes.
        #[arg(long, alias = "est")]
        estimate: Option<String>,
        /// Replace metadata sub-bullet with raw metadata text.
        #[arg(long)]
        metadata: Option<String>,
        /// Clear metadata key: area,priority,energy,weight,due,start,estimate.
        #[arg(long)]
        clear: Vec<String>,
    },
    /// Archive completed todos to life-log by week and area.
    Archive,
}

#[derive(Debug, Subcommand)]
enum TuiCommand {
    /// Open the workspace on the notes tab.
    Notes,
    /// Open the workspace on the todos tab.
    Todos,
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command.unwrap_or(Command::Tui { command: None }) {
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
        Command::Get { id, search, tags } => notes_get_command(id, search, tags),
        Command::Seal => sync_notes_command(),
        Command::Ingest { prune } => ingest_notes_command(prune),
        Command::Format { all } => format_notes_command(all),
        Command::Register => register_notes_command(),
        Command::Validate { all, domain } => validate_notes_command(all, domain),
        Command::ValidateRegistry => validate_registry_command(),
        Command::Graph {
            search,
            domain,
            depth,
            limit,
        } => notes_graph_command(search, domain, depth, limit),
        Command::Query { command } => query_command(command),
        Command::Todos { command } => todos_command(command),
        Command::Tui { command } => tui_command(command),
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

fn tui_command(command: Option<TuiCommand>) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    let notes = db
        .list_notes(250, 0)?
        .into_iter()
        .map(|note| mw_tui::WorkspaceNote {
            id: note.id,
            title: note.title,
            path: note.path,
            uid: note.uid,
            domains: note.domains,
            tags: note.tags,
        })
        .collect();
    let todos = if Path::new(&cfg.notes_dir).is_dir() {
        mw_notes::list_active_task_index_todos(&cfg.notes_dir)
            .unwrap_or_default()
            .0
            .into_iter()
            .map(|todo| mw_tui::WorkspaceTodo {
                id: todo.id,
                title: todo.text,
                area: todo.area,
                status: todo.metadata.status,
                source_path: todo.source_path,
                priority: todo.metadata.priority,
                done: todo.done,
            })
            .collect()
    } else {
        Vec::new()
    };
    let initial_tab = match command.unwrap_or(TuiCommand::Notes) {
        TuiCommand::Notes => mw_tui::WorkspaceInitialTab::Notes,
        TuiCommand::Todos => mw_tui::WorkspaceInitialTab::Todos,
    };
    mw_tui::run_workspace_with_tab(
        mw_tui::WorkspaceData {
            notes,
            todos,
            notes_dir: cfg.notes_dir,
            db_path: cfg.db_path,
        },
        initial_tab,
    )
}

fn todos_command(command: TodosCommand) -> Result<()> {
    match command {
        TodosCommand::Sync => todos_sync_command(),
        TodosCommand::Toggle { id } => todos_toggle_command(&id),
        TodosCommand::Inspect { id } => todos_inspect_command(&id),
        TodosCommand::Update {
            id,
            title,
            area,
            priority,
            energy,
            weight,
            due,
            start,
            estimate,
            metadata,
            clear,
        } => todos_update_command(mw_notes::TodoUpdateParams {
            ids: id,
            title,
            area,
            priority,
            energy,
            weight,
            due,
            start,
            estimate,
            metadata,
            clear,
        }),
        TodosCommand::Archive => todos_archive_command(),
    }
}

fn todos_update_command(params: mw_notes::TodoUpdateParams) -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }
    let updated = mw_notes::update_task_index_todos(root, &cfg.dashboard_path, params)
        .context("update todos")?;
    let ids = updated
        .iter()
        .map(|todo| todo.id.as_str())
        .collect::<Vec<_>>()
        .join(", ");
    println!("✅ updated {} todo(s): {}", updated.len(), ids);
    Ok(())
}

fn todos_toggle_command(id: &str) -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }
    let (todo, done) =
        mw_notes::toggle_task_index_todo(root, &cfg.dashboard_path, id).context("toggle todo")?;
    let state = if done { "done" } else { "open" };
    println!(
        "✅ toggled todo {:?} to {} ({}:{})",
        todo.text, state, todo.source_path, todo.line
    );
    Ok(())
}

fn todos_inspect_command(id: &str) -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }
    let todo = mw_notes::get_active_task_index_todo(root, id).context("inspect todo")?;
    write_json(&todo)
}

fn todos_archive_command() -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    println!("🔁 syncing dashboard selections back to source notes before archive");
    mw_notes::sync_dashboard_from_task_index_notes(root, &cfg.dashboard_path)
        .context("sync dashboard before archive")?;

    let stats = mw_notes::archive_completed_to_life_log(root).context("archive completed todos")?;
    if stats.archived_tasks == 0 {
        println!("📦 no completed todos found to archive");
        return Ok(());
    }

    println!(
        "📦 archived {} completed task(s) from {} active task-index note(s)",
        stats.archived_tasks, stats.active_task_index_notes
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
        if let Some(n) = stats.archived_by_area.get(group).filter(|n| **n > 0) {
            println!("  • {group}: {n}");
        }
    }
    println!(
        "🗂 updated {} life-log month file(s), pruned {} source note(s)",
        stats.month_files_updated, stats.source_files_updated
    );
    println!("🔁 refreshing dashboard projection after archive");
    todos_sync_command()
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
        QueryCommand::Graph {
            search,
            domain,
            depth,
            limit,
        } => query_graph_command(search, domain, depth, limit),
        QueryCommand::Recipes { scope } => query_recipes_command(scope),
        QueryCommand::Projection { projection, scope } => {
            query_projection_command(&projection, scope)
        }
        QueryCommand::Ingredients {
            mentions,
            unresolved,
        } => query_ingredients_command(mentions, unresolved),
        QueryCommand::Registry => query_registry_command(),
    }
}

#[derive(Debug, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct RegistryQueryResult {
    note_ids: Vec<mw_db::RegistryEntryRecord>,
    conflicts: Vec<mw_db::RegistryConflictRecord>,
}

fn query_registry_command() -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    write_json(&RegistryQueryResult {
        note_ids: db.list_registry_entries()?,
        conflicts: db.list_registry_conflicts()?,
    })
}

fn query_graph_command(
    search: Option<String>,
    domain: Option<String>,
    depth: i64,
    limit: i64,
) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    write_json(&db.query_graph(
        &search.unwrap_or_default(),
        &domain.unwrap_or_default(),
        depth,
        limit,
    )?)
}

fn query_projection_command(projection: &str, scope: Vec<String>) -> Result<()> {
    match projection.trim().to_ascii_lowercase().as_str() {
        "recipe" | "recipes" => query_recipes_command(scope),
        "" => bail!(
            "projection name is required (example: mw query projection recipe --scope cooking)"
        ),
        other => bail!("unsupported projection {other:?}"),
    }
}

fn query_recipes_command(scope: Vec<String>) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    write_json(&db.list_recipes(&split_repeated_csv(scope))?)
}

fn query_ingredients_command(mentions: bool, unresolved: bool) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    if mentions || unresolved {
        return write_json(&db.list_ingredient_mentions(unresolved)?);
    }
    write_json(&db.list_ingredients()?)
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

fn split_repeated_csv(values: Vec<String>) -> Vec<String> {
    values.iter().flat_map(|value| split_csv(value)).collect()
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
        NotesCommand::Sync => sync_notes_command(),
        NotesCommand::Get { id, search, tags } => notes_get_command(id, search, tags),
        NotesCommand::Format { all } => format_notes_command(all),
        NotesCommand::Ingest { prune } => ingest_notes_command(prune),
        NotesCommand::Register => register_notes_command(),
        NotesCommand::Validate { all, domain } => validate_notes_command(all, domain),
        NotesCommand::ValidateRegistry => validate_registry_command(),
        NotesCommand::Graph {
            search,
            domain,
            depth,
            limit,
        } => notes_graph_command(search, domain, depth, limit),
    }
}

fn notes_graph_command(
    search: Option<String>,
    domain: Option<String>,
    depth: i64,
    limit: i64,
) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    let graph = db.query_graph(
        &search.clone().unwrap_or_default(),
        &domain.clone().unwrap_or_default(),
        depth,
        limit,
    )?;
    mw_tui::run_graph_browser(mw_tui::GraphBrowserData {
        nodes: graph
            .nodes
            .into_iter()
            .map(|node| mw_tui::GraphBrowserNode {
                id: node.id,
                label: node.label,
                title: node.title,
                path: node.path,
                tags: node.tags,
                domains: node.domains,
                matched: node.matched,
                unknown: node.unknown,
            })
            .collect(),
        edges: graph
            .edges
            .into_iter()
            .map(|edge| mw_tui::GraphBrowserEdge {
                source: edge.source,
                target: edge.target,
                label: edge.label,
                kind: edge.kind,
            })
            .collect(),
        search: search.unwrap_or_default(),
        domain: domain.unwrap_or_default(),
        depth,
    })
}

fn notes_get_command(id: Option<i64>, search: Option<String>, tags: Option<String>) -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;

    if let Some(id) = id.filter(|id| *id != 0) {
        let notes = db.get_note_by_id(id)?.into_iter().collect::<Vec<_>>();
        return write_get_notes_pretty(&notes);
    }

    if let Some(search) = trim_non_empty(search) {
        return write_get_notes_pretty(&db.search_notes_by_title(&search)?);
    }

    if let Some(tags) = trim_non_empty(tags) {
        return write_get_notes_pretty(&db.list_notes_by_tags(&split_csv(&tags))?);
    }

    write_get_notes_pretty(&db.list_notes(50, 0)?)
}

fn write_get_notes_pretty(notes: &[mw_db::NoteRecord]) -> Result<()> {
    for note in notes {
        println!("📄 {}", note.title);
        println!("Path: {}", note.path);
        if !note.uid.is_empty() {
            println!("UID: {}", note.uid);
        }
        println!("Tags: {:?}", note.tags);
        let links = note
            .links
            .iter()
            .map(|link| link.target.as_str())
            .collect::<Vec<_>>();
        println!("Links: {:?}", links);
        println!("---");
    }
    Ok(())
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
struct IngestStats {
    ingested: usize,
    pruned: usize,
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
struct RegistryStats {
    registered: usize,
    conflicts: usize,
    blocking_conflicts: usize,
}

fn sync_notes_command() -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    println!("🧙 Running notes pipeline: format → ingest → register → validate-registry");

    let format = format_notes(&cfg, true)?;
    println!(
        "✅ Done. Updated {} hub file(s) and {} note file(s).",
        format.hub_files_updated, format.note_files_updated
    );
    for issue in format.issues {
        println!("⚠️  {issue}");
    }

    let ingest = ingest_notes(&cfg, true)?;
    println!("✅ Pruned {} notes", ingest.pruned);
    println!("✅ ingested {} notes", ingest.ingested);

    let registry = register_notes(&cfg)?;
    println!(
        "✅ Registry updated: {} registered, {} conflict(s)",
        registry.registered, registry.conflicts
    );
    validate_registry_stats(registry)?;
    println!("✅ Notes have been ingested, registered, and registry-validated");
    Ok(())
}

fn format_notes_command(all: bool) -> Result<()> {
    let cfg = config::load()?;
    let stats = format_notes(&cfg, all)?;
    if all {
        println!(
            "✅ Done. Updated {} hub file(s) and {} note file(s).",
            stats.hub_files_updated, stats.note_files_updated
        );
    } else {
        println!("✅ Done. Updated {} hub file(s).", stats.hub_files_updated);
    }
    for issue in stats.issues {
        println!("⚠️  {issue}");
    }
    Ok(())
}

fn format_notes(cfg: &mw_core::config::Config, all: bool) -> Result<mw_notes::FormatStats> {
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }
    mw_notes::format_notes(root, all).context("format notes")
}

fn validate_registry_stats(stats: RegistryStats) -> Result<()> {
    if stats.blocking_conflicts > 0 {
        bail!(
            "registry validation failed: {} blocking conflict(s)",
            stats.blocking_conflicts
        );
    }
    println!("✅ Registry validation passed");
    Ok(())
}

fn validate_notes_command(_all: bool, domain: Option<String>) -> Result<()> {
    let cfg = config::load()?;
    let root = Path::new(&cfg.notes_dir);
    if !root.is_dir() {
        bail!("notes directory missing: {}", cfg.notes_dir);
    }

    println!("🧪 Validating notes...");
    let result = mw_notes::build_registry(root).context("scan notes on disk")?;
    let mut has_error = false;
    for rel in &result.missing_hub {
        has_error = true;
        println!("[ERROR] {rel}  MISSING_HUB_ID");
    }
    for duplicate in &result.duplicates {
        has_error = true;
        for rel in &duplicate.paths {
            println!("[ERROR] {rel}  DUPLICATE_ID {}", duplicate.id);
        }
    }

    if let Some(domain) = trim_non_empty(domain) {
        let canonical_domain = mw_notes::canonical_domain_name(&domain)
            .map_err(|err| anyhow::anyhow!(err))
            .with_context(|| format!("load domain schema {domain:?}"))?;
        let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
        let notes = db
            .list_notes_by_domain(&canonical_domain)
            .with_context(|| format!("query notes for domain {canonical_domain:?}"))?;
        let result = mw_notes::validate_domain_notes(
            &domain,
            notes
                .iter()
                .map(|note| (note.path.as_str(), note.content.as_str())),
        )
        .map_err(|err| anyhow::anyhow!(err))
        .with_context(|| format!("validate domain {domain:?}"))?;
        if !result.violations.is_empty() {
            write_json(&result.violations)?;
        }
        if result.has_errors {
            has_error = true;
        }
        println!(
            "🧪 Domain validation: {} note(s) checked for {}",
            result.checked, result.domain
        );
    }

    finish_validation(has_error)
}

fn validate_registry_command() -> Result<()> {
    let cfg = config::load()?;
    let db = mw_db::NoteDb::open(&cfg.db_path, Some(&cfg.notes_schema_path))?;
    println!("🧪 Validating registry conflicts...");
    let conflicts = db.list_registry_conflicts()?;
    let mut has_error = false;
    for conflict in &conflicts {
        let severity = registry_conflict_severity(&conflict.reason);
        if severity == "ERROR" {
            has_error = true;
        }
        let uid = conflict.uid.clone().unwrap_or_default();
        println!(
            "[{severity}] {}  {} {}",
            conflict.path, conflict.reason, uid
        );
    }
    finish_validation(has_error)
}

fn registry_conflict_severity(reason: &str) -> &'static str {
    match reason.trim() {
        "MISSING_HUB_ID" | "DUPLICATE_ID" => "ERROR",
        _ => "WARN",
    }
}

fn finish_validation(has_error: bool) -> Result<()> {
    if has_error {
        bail!("❌ Validation failed");
    }
    println!("✅ Validation passed.");
    Ok(())
}

fn register_notes_command() -> Result<()> {
    let cfg = config::load()?;
    let stats = register_notes(&cfg)?;
    println!(
        "✅ Registry updated: {} registered, {} conflict(s)",
        stats.registered, stats.conflicts
    );
    Ok(())
}

fn register_notes(cfg: &mw_core::config::Config) -> Result<RegistryStats> {
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
    let blocking_conflicts = conflicts
        .iter()
        .filter(|conflict| matches!(conflict.reason.as_str(), "MISSING_HUB_ID" | "DUPLICATE_ID"))
        .count();
    Ok(RegistryStats {
        registered: entries.len(),
        conflicts: conflicts.len(),
        blocking_conflicts,
    })
}

fn ingest_notes_command(prune: bool) -> Result<()> {
    let cfg = config::load()?;
    let stats = ingest_notes(&cfg, prune)?;
    println!("✅ Pruned {} notes", stats.pruned);
    println!("✅ ingested {} notes", stats.ingested);
    Ok(())
}

fn ingest_notes(cfg: &mw_core::config::Config, prune: bool) -> Result<IngestStats> {
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
    if prune {
        for path in db.all_note_paths()? {
            if !disk_paths.contains(&path) {
                db.delete_note_by_path(&path)
                    .with_context(|| format!("prune {path}"))?;
                pruned += 1;
            }
        }
    }

    Ok(IngestStats {
        ingested: count,
        pruned,
    })
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
