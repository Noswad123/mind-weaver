use anyhow::{Result, bail};
use clap::{Parser, Subcommand};
use mw_core::config::{self, CheckStatus, InitOptions};

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
