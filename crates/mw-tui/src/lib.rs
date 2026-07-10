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
    style::Style,
    text::{Line, Span},
    widgets::{List, ListItem, ListState, Paragraph, Wrap},
};

mod catppuccin {
    use ratatui::{
        style::{Color, Modifier, Style},
        widgets::{Block, Borders},
    };

    pub const YELLOW: Color = Color::Rgb(249, 226, 175);
    pub const GREEN: Color = Color::Rgb(166, 227, 161);
    pub const SKY: Color = Color::Rgb(137, 220, 235);
    pub const MAUVE: Color = Color::Rgb(203, 166, 247);
    pub const LAVENDER: Color = Color::Rgb(180, 190, 254);
    pub const TEXT: Color = Color::Rgb(205, 214, 244);
    pub const SUBTEXT: Color = Color::Rgb(186, 194, 222);
    pub const OVERLAY: Color = Color::Rgb(108, 112, 134);
    pub const SURFACE: Color = Color::Rgb(49, 50, 68);
    pub const BASE: Color = Color::Rgb(30, 30, 46);

    pub fn app() -> Style {
        Style::default().fg(TEXT).bg(BASE)
    }

    pub fn title() -> Style {
        Style::default()
            .fg(SKY)
            .bg(BASE)
            .add_modifier(Modifier::BOLD)
    }

    pub fn subtitle() -> Style {
        Style::default().fg(SUBTEXT).bg(BASE)
    }

    pub fn help() -> Style {
        Style::default().fg(OVERLAY).bg(BASE)
    }

    pub fn highlight() -> Style {
        Style::default()
            .fg(BASE)
            .bg(MAUVE)
            .add_modifier(Modifier::BOLD)
    }

    pub fn label() -> Style {
        Style::default()
            .fg(LAVENDER)
            .bg(BASE)
            .add_modifier(Modifier::BOLD)
    }

    pub fn block(title: &'static str) -> Block<'static> {
        Block::default()
            .title(title)
            .borders(Borders::ALL)
            .border_style(Style::default().fg(SURFACE).bg(BASE))
            .style(app())
    }

    pub fn bare_block() -> Block<'static> {
        Block::default()
            .borders(Borders::ALL)
            .border_style(Style::default().fg(SURFACE).bg(BASE))
            .style(app())
    }
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct GraphBrowserData {
    pub nodes: Vec<GraphBrowserNode>,
    pub edges: Vec<GraphBrowserEdge>,
    pub search: String,
    pub domain: String,
    pub depth: i64,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct GraphBrowserNode {
    pub id: String,
    pub label: String,
    pub title: String,
    pub path: String,
    pub tags: Vec<String>,
    pub domains: Vec<String>,
    pub matched: bool,
    pub unknown: bool,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct GraphBrowserEdge {
    pub source: String,
    pub target: String,
    pub label: String,
    pub kind: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct WorkspaceData {
    pub notes: Vec<WorkspaceNote>,
    pub todos: Vec<WorkspaceTodo>,
    pub notes_dir: String,
    pub db_path: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct WorkspaceNote {
    pub id: i64,
    pub title: String,
    pub path: String,
    pub uid: String,
    pub domains: Vec<String>,
    pub tags: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct WorkspaceTodo {
    pub id: String,
    pub title: String,
    pub area: String,
    pub status: String,
    pub source_path: String,
    pub priority: String,
    pub done: bool,
}

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub enum WorkspaceInitialTab {
    #[default]
    Notes,
    Todos,
}

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

pub fn run_workspace(data: WorkspaceData) -> Result<()> {
    run_workspace_with_tab(data, WorkspaceInitialTab::Notes)
}

pub fn run_workspace_with_tab(data: WorkspaceData, initial_tab: WorkspaceInitialTab) -> Result<()> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;

    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;
    let result = run_workspace_app(&mut terminal, data, initial_tab);

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    terminal.show_cursor()?;

    result
}

pub fn run_graph_browser(data: GraphBrowserData) -> Result<()> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;

    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;
    let result = run_graph_app(&mut terminal, data);

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
                Span::styled("MindWeaver", catppuccin::title()),
                Span::styled(" Rust port", catppuccin::subtitle()),
            ]))
            .alignment(Alignment::Center)
            .style(catppuccin::app())
            .block(catppuccin::bare_block());
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
            .style(catppuccin::app())
            .block(catppuccin::block("Workspace"));
            frame.render_widget(body, chunks[1]);

            let help = Paragraph::new("q / Esc / Ctrl-C: quit")
                .style(catppuccin::help())
                .alignment(Alignment::Center)
                .block(catppuccin::bare_block());
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

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
enum WorkspaceTab {
    #[default]
    Notes,
    Todos,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct WorkspaceState {
    data: WorkspaceData,
    tab: WorkspaceTab,
    note_selected: usize,
    todo_selected: usize,
}

impl WorkspaceState {
    fn new_with_tab(data: WorkspaceData, initial_tab: WorkspaceInitialTab) -> Self {
        Self {
            data,
            tab: match initial_tab {
                WorkspaceInitialTab::Notes => WorkspaceTab::Notes,
                WorkspaceInitialTab::Todos => WorkspaceTab::Todos,
            },
            note_selected: 0,
            todo_selected: 0,
        }
    }

    fn switch_tab(&mut self) {
        self.tab = match self.tab {
            WorkspaceTab::Notes => WorkspaceTab::Todos,
            WorkspaceTab::Todos => WorkspaceTab::Notes,
        };
    }

    fn move_up(&mut self) {
        match self.tab {
            WorkspaceTab::Notes => self.note_selected = self.note_selected.saturating_sub(1),
            WorkspaceTab::Todos => self.todo_selected = self.todo_selected.saturating_sub(1),
        }
    }

    fn move_down(&mut self) {
        match self.tab {
            WorkspaceTab::Notes => {
                if self.note_selected + 1 < self.data.notes.len() {
                    self.note_selected += 1;
                }
            }
            WorkspaceTab::Todos => {
                if self.todo_selected + 1 < self.data.todos.len() {
                    self.todo_selected += 1;
                }
            }
        }
    }

    fn selected_note(&self) -> Option<&WorkspaceNote> {
        self.data.notes.get(self.note_selected)
    }

    fn selected_todo(&self) -> Option<&WorkspaceTodo> {
        self.data.todos.get(self.todo_selected)
    }
}

fn run_workspace_app(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    data: WorkspaceData,
    initial_tab: WorkspaceInitialTab,
) -> Result<()> {
    let mut state = WorkspaceState::new_with_tab(data, initial_tab);
    loop {
        terminal.draw(|frame| {
            let chunks = Layout::default()
                .direction(Direction::Vertical)
                .constraints([
                    Constraint::Length(3),
                    Constraint::Min(8),
                    Constraint::Length(3),
                ])
                .split(frame.area());

            let title = Paragraph::new(Line::from(vec![
                Span::styled("MindWeaver Workspace", catppuccin::title()),
                Span::styled(
                    format!(
                        " — {} note(s), {} todo(s)",
                        state.data.notes.len(),
                        state.data.todos.len()
                    ),
                    catppuccin::subtitle(),
                ),
            ]))
            .alignment(Alignment::Center)
            .style(catppuccin::app())
            .block(catppuccin::bare_block());
            frame.render_widget(title, chunks[0]);

            let body = Layout::default()
                .direction(Direction::Horizontal)
                .constraints([Constraint::Percentage(42), Constraint::Percentage(58)])
                .split(chunks[1]);

            let mut list_state = list_state_for_workspace(&state);
            frame.render_stateful_widget(render_workspace_list(&state), body[0], &mut list_state);
            frame.render_widget(render_workspace_details(&state), body[1]);

            let help =
                Paragraph::new("Tab: notes/todos • ↑/↓ or k/j: select • q / Esc / Ctrl-C: quit")
                    .style(catppuccin::help())
                    .alignment(Alignment::Center)
                    .block(catppuccin::bare_block());
            frame.render_widget(help, chunks[2]);
        })?;

        if let Event::Key(key) = event::read()? {
            let should_quit = matches!(key.code, KeyCode::Char('q') | KeyCode::Esc)
                || (matches!(key.code, KeyCode::Char('c'))
                    && key.modifiers.contains(event::KeyModifiers::CONTROL));
            if should_quit {
                return Ok(());
            }
            match key.code {
                KeyCode::Tab | KeyCode::Left | KeyCode::Right => state.switch_tab(),
                KeyCode::Up | KeyCode::Char('k') => state.move_up(),
                KeyCode::Down | KeyCode::Char('j') => state.move_down(),
                _ => {}
            }
        }
    }
}

fn list_state_for_workspace(state: &WorkspaceState) -> ListState {
    let mut list_state = ListState::default();
    match state.tab {
        WorkspaceTab::Notes if !state.data.notes.is_empty() => {
            list_state.select(Some(state.note_selected));
        }
        WorkspaceTab::Todos if !state.data.todos.is_empty() => {
            list_state.select(Some(state.todo_selected));
        }
        _ => {}
    }
    list_state
}

fn render_workspace_list(state: &WorkspaceState) -> List<'_> {
    match state.tab {
        WorkspaceTab::Notes => {
            let items: Vec<ListItem> = state
                .data
                .notes
                .iter()
                .map(|note| {
                    let label = if note.uid.trim().is_empty() {
                        note.title.as_str()
                    } else {
                        note.uid.as_str()
                    };
                    ListItem::new(Line::from(vec![
                        Span::styled("📝 ", Style::default().fg(catppuccin::SKY)),
                        Span::styled(label.to_string(), catppuccin::app()),
                    ]))
                })
                .collect();
            List::new(items)
                .block(catppuccin::block("Notes"))
                .style(catppuccin::app())
                .highlight_style(catppuccin::highlight())
                .highlight_symbol("> ")
        }
        WorkspaceTab::Todos => {
            let items: Vec<ListItem> = state
                .data
                .todos
                .iter()
                .map(|todo| {
                    let checkbox = if todo.done { "[x]" } else { "[ ]" };
                    let checkbox_style = if todo.done {
                        Style::default().fg(catppuccin::GREEN).bg(catppuccin::BASE)
                    } else {
                        Style::default()
                            .fg(catppuccin::OVERLAY)
                            .bg(catppuccin::BASE)
                    };
                    ListItem::new(Line::from(vec![
                        Span::styled(checkbox.to_string(), checkbox_style),
                        Span::raw(" "),
                        Span::styled(todo.title.clone(), catppuccin::app()),
                        Span::styled(format!(" ({})", todo.area), catppuccin::subtitle()),
                    ]))
                })
                .collect();
            List::new(items)
                .block(catppuccin::block("Todos"))
                .style(catppuccin::app())
                .highlight_style(catppuccin::highlight())
                .highlight_symbol("> ")
        }
    }
}

fn render_workspace_details(state: &WorkspaceState) -> Paragraph<'_> {
    let mut lines = Vec::new();
    match state.tab {
        WorkspaceTab::Notes => {
            if let Some(note) = state.selected_note() {
                lines.push(Line::from(vec![
                    Span::styled("Note", catppuccin::title()),
                    Span::styled(format!(" #{}", note.id), catppuccin::subtitle()),
                ]));
                lines.push(labeled_line("Title", &note.title));
                if !note.uid.trim().is_empty() {
                    lines.push(labeled_line("UID", &note.uid));
                }
                lines.push(labeled_line("Path", &note.path));
                if !note.domains.is_empty() {
                    lines.push(labeled_line("Domains", &note.domains.join(", ")));
                }
                if !note.tags.is_empty() {
                    lines.push(labeled_line("Tags", &note.tags.join(", ")));
                }
            } else {
                lines.push(Line::from(Span::styled(
                    "No notes indexed. Run `mw notes ingest`.",
                    Style::default().fg(catppuccin::YELLOW).bg(catppuccin::BASE),
                )));
            }
        }
        WorkspaceTab::Todos => {
            if let Some(todo) = state.selected_todo() {
                lines.push(Line::from(vec![
                    Span::styled("Todo", catppuccin::title()),
                    Span::styled(format!(" {}", todo.id), catppuccin::subtitle()),
                ]));
                lines.push(labeled_line("Title", &todo.title));
                lines.push(labeled_line("Area", &todo.area));
                lines.push(labeled_line("Status", &todo.status));
                if !todo.priority.trim().is_empty() {
                    lines.push(labeled_line("Priority", &todo.priority));
                }
                lines.push(labeled_line("Source", &todo.source_path));
            } else {
                lines.push(Line::from(Span::styled(
                    "No active task-index todos found.",
                    Style::default().fg(catppuccin::YELLOW).bg(catppuccin::BASE),
                )));
            }
        }
    }
    lines.push(Line::from(""));
    lines.push(labeled_line("Notes dir", &state.data.notes_dir));
    lines.push(labeled_line("DB", &state.data.db_path));

    Paragraph::new(lines)
        .style(catppuccin::app())
        .block(catppuccin::block("Details"))
        .wrap(Wrap { trim: false })
}

fn labeled_line<'a>(label: &'static str, value: impl Into<String>) -> Line<'a> {
    Line::from(vec![
        Span::styled(format!("{label}: "), catppuccin::label()),
        Span::styled(value.into(), catppuccin::app()),
    ])
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
struct GraphBrowserState {
    data: GraphBrowserData,
    selected: usize,
}

impl GraphBrowserState {
    fn new(data: GraphBrowserData) -> Self {
        Self { data, selected: 0 }
    }

    fn move_up(&mut self) {
        self.selected = self.selected.saturating_sub(1);
    }

    fn move_down(&mut self) {
        if self.selected + 1 < self.data.nodes.len() {
            self.selected += 1;
        }
    }

    fn selected_node(&self) -> Option<&GraphBrowserNode> {
        self.data.nodes.get(self.selected)
    }

    fn selected_edges(&self) -> Vec<&GraphBrowserEdge> {
        let Some(node) = self.selected_node() else {
            return Vec::new();
        };
        self.data
            .edges
            .iter()
            .filter(|edge| edge.source == node.id || edge.target == node.id)
            .collect()
    }
}

fn run_graph_app(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    data: GraphBrowserData,
) -> Result<()> {
    let mut state = GraphBrowserState::new(data);
    loop {
        terminal.draw(|frame| {
            let chunks = Layout::default()
                .direction(Direction::Vertical)
                .constraints([
                    Constraint::Length(3),
                    Constraint::Min(8),
                    Constraint::Length(3),
                ])
                .split(frame.area());

            let title = format!(
                "Graph Browser — {} node(s), {} edge(s), depth {}{}{}",
                state.data.nodes.len(),
                state.data.edges.len(),
                state.data.depth,
                if state.data.search.trim().is_empty() {
                    "".to_string()
                } else {
                    format!(" search={}", state.data.search)
                },
                if state.data.domain.trim().is_empty() {
                    "".to_string()
                } else {
                    format!(" domain={}", state.data.domain)
                }
            );
            frame.render_widget(
                Paragraph::new(title)
                    .style(catppuccin::title())
                    .alignment(Alignment::Center)
                    .block(catppuccin::bare_block()),
                chunks[0],
            );

            let body = Layout::default()
                .direction(Direction::Horizontal)
                .constraints([Constraint::Percentage(42), Constraint::Percentage(58)])
                .split(chunks[1]);

            let items: Vec<ListItem> = state
                .data
                .nodes
                .iter()
                .map(|node| {
                    let marker = if node.matched { "●" } else { "○" };
                    let label = if node.label.trim().is_empty() {
                        node.title.as_str()
                    } else {
                        node.label.as_str()
                    };
                    let marker_style = if node.matched {
                        Style::default().fg(catppuccin::GREEN).bg(catppuccin::BASE)
                    } else {
                        Style::default()
                            .fg(catppuccin::OVERLAY)
                            .bg(catppuccin::BASE)
                    };
                    ListItem::new(Line::from(vec![
                        Span::styled(marker.to_string(), marker_style),
                        Span::raw(" "),
                        Span::styled(label.to_string(), catppuccin::app()),
                    ]))
                })
                .collect();
            let mut list_state = ListState::default();
            if !state.data.nodes.is_empty() {
                list_state.select(Some(state.selected));
            }
            let list = List::new(items)
                .block(catppuccin::block("Nodes"))
                .style(catppuccin::app())
                .highlight_style(catppuccin::highlight())
                .highlight_symbol("> ");
            frame.render_stateful_widget(list, body[0], &mut list_state);

            frame.render_widget(render_graph_details(&state), body[1]);

            let help = Paragraph::new("↑/↓ or k/j: select • q / Esc / Ctrl-C: quit")
                .style(catppuccin::help())
                .alignment(Alignment::Center)
                .block(catppuccin::bare_block());
            frame.render_widget(help, chunks[2]);
        })?;

        if let Event::Key(key) = event::read()? {
            let should_quit = matches!(key.code, KeyCode::Char('q') | KeyCode::Esc)
                || (matches!(key.code, KeyCode::Char('c'))
                    && key.modifiers.contains(event::KeyModifiers::CONTROL));
            if should_quit {
                return Ok(());
            }
            match key.code {
                KeyCode::Up | KeyCode::Char('k') => state.move_up(),
                KeyCode::Down | KeyCode::Char('j') => state.move_down(),
                _ => {}
            }
        }
    }
}

fn render_graph_details(state: &GraphBrowserState) -> Paragraph<'_> {
    let mut lines = Vec::new();
    let Some(node) = state.selected_node() else {
        return Paragraph::new(
            "No nodes. Run `mw notes ingest` and `mw notes register`, then try again.",
        )
        .style(Style::default().fg(catppuccin::YELLOW).bg(catppuccin::BASE))
        .block(catppuccin::block("Details"))
        .wrap(Wrap { trim: false });
    };

    lines.push(labeled_line("Label", node.label.clone()));
    lines.push(labeled_line("Title", node.title.clone()));
    lines.push(labeled_line("Path", node.path.clone()));
    if !node.domains.is_empty() {
        lines.push(labeled_line("Domains", node.domains.join(", ")));
    }
    if !node.tags.is_empty() {
        lines.push(labeled_line("Tags", node.tags.join(", ")));
    }
    lines.push(Line::from(""));
    lines.push(Line::from(Span::styled(
        "Connected edges",
        catppuccin::title(),
    )));

    let connected = state.selected_edges();
    if connected.is_empty() {
        lines.push(Line::from(Span::styled("  None", catppuccin::help())));
    } else {
        for edge in connected.into_iter().take(20) {
            let arrow = if edge.source == node.id { "→" } else { "←" };
            let other = if edge.source == node.id {
                edge.target.as_str()
            } else {
                edge.source.as_str()
            };
            lines.push(Line::from(vec![
                Span::styled(
                    format!("  {arrow} "),
                    Style::default().fg(catppuccin::SKY).bg(catppuccin::BASE),
                ),
                Span::styled(other.to_string(), catppuccin::app()),
                Span::styled(format!("  {}", edge.label), catppuccin::subtitle()),
            ]));
        }
    }

    Paragraph::new(lines)
        .style(catppuccin::app())
        .block(catppuccin::block("Details"))
        .wrap(Wrap { trim: false })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn graph_state_selection_is_bounded() {
        let mut state = GraphBrowserState::new(GraphBrowserData {
            nodes: vec![
                GraphBrowserNode {
                    id: "note:1".to_string(),
                    label: "one".to_string(),
                    ..GraphBrowserNode::default()
                },
                GraphBrowserNode {
                    id: "note:2".to_string(),
                    label: "two".to_string(),
                    ..GraphBrowserNode::default()
                },
            ],
            ..GraphBrowserData::default()
        });
        state.move_up();
        assert_eq!(state.selected, 0);
        state.move_down();
        state.move_down();
        assert_eq!(state.selected, 1);
    }

    #[test]
    fn graph_state_finds_selected_edges() {
        let state = GraphBrowserState::new(GraphBrowserData {
            nodes: vec![GraphBrowserNode {
                id: "note:1".to_string(),
                label: "one".to_string(),
                ..GraphBrowserNode::default()
            }],
            edges: vec![GraphBrowserEdge {
                source: "note:1".to_string(),
                target: "note:2".to_string(),
                label: "Two".to_string(),
                kind: "mentions".to_string(),
            }],
            ..GraphBrowserData::default()
        });
        assert_eq!(state.selected_edges().len(), 1);
    }

    #[test]
    fn workspace_state_switches_tabs_and_bounds_selection() {
        let mut state = WorkspaceState::new_with_tab(
            WorkspaceData {
                notes: vec![
                    WorkspaceNote {
                        title: "one".to_string(),
                        ..WorkspaceNote::default()
                    },
                    WorkspaceNote {
                        title: "two".to_string(),
                        ..WorkspaceNote::default()
                    },
                ],
                todos: vec![WorkspaceTodo {
                    title: "todo".to_string(),
                    ..WorkspaceTodo::default()
                }],
                ..WorkspaceData::default()
            },
            WorkspaceInitialTab::Notes,
        );

        state.move_down();
        state.move_down();
        assert_eq!(state.note_selected, 1);
        state.switch_tab();
        assert_eq!(state.tab, WorkspaceTab::Todos);
        state.move_down();
        assert_eq!(state.todo_selected, 0);
        assert_eq!(state.selected_todo().unwrap().title, "todo");
    }

    #[test]
    fn workspace_state_can_start_on_todos_tab() {
        let state = WorkspaceState::new_with_tab(
            WorkspaceData {
                todos: vec![WorkspaceTodo {
                    title: "todo".to_string(),
                    ..WorkspaceTodo::default()
                }],
                ..WorkspaceData::default()
            },
            WorkspaceInitialTab::Todos,
        );

        assert_eq!(state.tab, WorkspaceTab::Todos);
        assert_eq!(state.selected_todo().unwrap().title, "todo");
    }
}
