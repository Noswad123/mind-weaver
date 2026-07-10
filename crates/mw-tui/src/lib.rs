use std::io;

use anyhow::Result;
use crossterm::{
    event::{self, Event, KeyCode},
    execute,
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{
    Terminal,
    backend::CrosstermBackend,
    layout::{Alignment, Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, Paragraph},
};

pub fn run() -> Result<()> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;

    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;
    let result = run_app(&mut terminal);

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    terminal.show_cursor()?;

    result
}

fn run_app(terminal: &mut Terminal<CrosstermBackend<io::Stdout>>) -> Result<()> {
    loop {
        terminal.draw(|frame| {
            let chunks = Layout::default()
                .direction(Direction::Vertical)
                .constraints([
                    Constraint::Length(3),
                    Constraint::Min(5),
                    Constraint::Length(3),
                ])
                .split(frame.area());

            let title = Paragraph::new(Line::from(vec![
                Span::styled(
                    "MindWeaver",
                    Style::default()
                        .fg(Color::Cyan)
                        .add_modifier(Modifier::BOLD),
                ),
                Span::raw(" Rust port"),
            ]))
            .alignment(Alignment::Center)
            .block(Block::default().borders(Borders::ALL));
            frame.render_widget(title, chunks[0]);

            let body = Paragraph::new(vec![
                Line::from("ratatui shell is online."),
                Line::from(""),
                Line::from("Next ports:"),
                Line::from("  • config/init/doctor"),
                Line::from("  • query projections"),
                Line::from("  • todos dashboard"),
                Line::from("  • graph browser"),
            ])
            .block(Block::default().title("Workspace").borders(Borders::ALL));
            frame.render_widget(body, chunks[1]);

            let help = Paragraph::new("q / Esc / Ctrl-C: quit")
                .style(Style::default().fg(Color::DarkGray))
                .alignment(Alignment::Center)
                .block(Block::default().borders(Borders::ALL));
            frame.render_widget(help, chunks[2]);
        })?;

        if let Event::Key(key) = event::read()? {
            let should_quit = matches!(key.code, KeyCode::Char('q') | KeyCode::Esc)
                || (matches!(key.code, KeyCode::Char('c'))
                    && key.modifiers.contains(event::KeyModifiers::CONTROL));
            if should_quit {
                return Ok(());
            }
        }
    }
}
