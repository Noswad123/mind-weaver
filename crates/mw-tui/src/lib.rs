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
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph, Wrap},
};

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
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;

    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;
    let result = run_workspace_app(&mut terminal, data);

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
    fn new(data: WorkspaceData) -> Self {
        Self {
            data,
            tab: WorkspaceTab::Notes,
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
) -> Result<()> {
    let mut state = WorkspaceState::new(data);
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
                Span::styled(
                    "MindWeaver Workspace",
                    Style::default()
                        .fg(Color::Cyan)
                        .add_modifier(Modifier::BOLD),
                ),
                Span::raw(format!(
                    " — {} note(s), {} todo(s)",
                    state.data.notes.len(),
                    state.data.todos.len()
                )),
            ]))
            .alignment(Alignment::Center)
            .block(Block::default().borders(Borders::ALL));
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
                    ListItem::new(format!("📝 {label}"))
                })
                .collect();
            List::new(items)
                .block(Block::default().title("Notes").borders(Borders::ALL))
                .highlight_style(
                    Style::default()
                        .fg(Color::Cyan)
                        .add_modifier(Modifier::BOLD),
                )
                .highlight_symbol("> ")
        }
        WorkspaceTab::Todos => {
            let items: Vec<ListItem> = state
                .data
                .todos
                .iter()
                .map(|todo| {
                    let checkbox = if todo.done { "[x]" } else { "[ ]" };
                    ListItem::new(format!("{checkbox} {} ({})", todo.title, todo.area))
                })
                .collect();
            List::new(items)
                .block(Block::default().title("Todos").borders(Borders::ALL))
                .highlight_style(
                    Style::default()
                        .fg(Color::Cyan)
                        .add_modifier(Modifier::BOLD),
                )
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
                    Span::styled(
                        "Note",
                        Style::default()
                            .fg(Color::Cyan)
                            .add_modifier(Modifier::BOLD),
                    ),
                    Span::raw(format!(" #{}", note.id)),
                ]));
                lines.push(Line::from(format!("Title: {}", note.title)));
                if !note.uid.trim().is_empty() {
                    lines.push(Line::from(format!("UID:   {}", note.uid)));
                }
                lines.push(Line::from(format!("Path:  {}", note.path)));
                if !note.domains.is_empty() {
                    lines.push(Line::from(format!("Domains: {}", note.domains.join(", "))));
                }
                if !note.tags.is_empty() {
                    lines.push(Line::from(format!("Tags: {}", note.tags.join(", "))));
                }
            } else {
                lines.push(Line::from("No notes indexed. Run `mw notes ingest`."));
            }
        }
        WorkspaceTab::Todos => {
            if let Some(todo) = state.selected_todo() {
                lines.push(Line::from(vec![
                    Span::styled(
                        "Todo",
                        Style::default()
                            .fg(Color::Cyan)
                            .add_modifier(Modifier::BOLD),
                    ),
                    Span::raw(format!(" {}", todo.id)),
                ]));
                lines.push(Line::from(format!("Title:  {}", todo.title)));
                lines.push(Line::from(format!("Area:   {}", todo.area)));
                lines.push(Line::from(format!("Status: {}", todo.status)));
                if !todo.priority.trim().is_empty() {
                    lines.push(Line::from(format!("Priority: {}", todo.priority)));
                }
                lines.push(Line::from(format!("Source: {}", todo.source_path)));
            } else {
                lines.push(Line::from("No active task-index todos found."));
            }
        }
    }
    lines.push(Line::from(""));
    lines.push(Line::from(format!("Notes dir: {}", state.data.notes_dir)));
    lines.push(Line::from(format!("DB: {}", state.data.db_path)));

    Paragraph::new(lines)
        .block(Block::default().title("Details").borders(Borders::ALL))
        .wrap(Wrap { trim: false })
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
                    .alignment(Alignment::Center)
                    .block(Block::default().borders(Borders::ALL)),
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
                    ListItem::new(format!("{marker} {label}"))
                })
                .collect();
            let mut list_state = ListState::default();
            if !state.data.nodes.is_empty() {
                list_state.select(Some(state.selected));
            }
            let list = List::new(items)
                .block(Block::default().title("Nodes").borders(Borders::ALL))
                .highlight_style(
                    Style::default()
                        .fg(Color::Cyan)
                        .add_modifier(Modifier::BOLD),
                )
                .highlight_symbol("> ");
            frame.render_stateful_widget(list, body[0], &mut list_state);

            frame.render_widget(render_graph_details(&state), body[1]);

            let help = Paragraph::new("↑/↓ or k/j: select • q / Esc / Ctrl-C: quit")
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
        .block(Block::default().title("Details").borders(Borders::ALL))
        .wrap(Wrap { trim: false });
    };

    lines.push(Line::from(vec![
        Span::styled("Label: ", Style::default().add_modifier(Modifier::BOLD)),
        Span::raw(node.label.clone()),
    ]));
    lines.push(Line::from(format!("Title: {}", node.title)));
    lines.push(Line::from(format!("Path:  {}", node.path)));
    if !node.domains.is_empty() {
        lines.push(Line::from(format!("Domains: {}", node.domains.join(", "))));
    }
    if !node.tags.is_empty() {
        lines.push(Line::from(format!("Tags: {}", node.tags.join(", "))));
    }
    lines.push(Line::from(""));
    lines.push(Line::from(Span::styled(
        "Connected edges",
        Style::default()
            .fg(Color::Cyan)
            .add_modifier(Modifier::BOLD),
    )));

    let connected = state.selected_edges();
    if connected.is_empty() {
        lines.push(Line::from("  None"));
    } else {
        for edge in connected.into_iter().take(20) {
            let arrow = if edge.source == node.id { "→" } else { "←" };
            let other = if edge.source == node.id {
                edge.target.as_str()
            } else {
                edge.source.as_str()
            };
            lines.push(Line::from(format!("  {arrow} {other}  {}", edge.label)));
        }
    }

    Paragraph::new(lines)
        .block(Block::default().title("Details").borders(Borders::ALL))
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
        let mut state = WorkspaceState::new(WorkspaceData {
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
        });

        state.move_down();
        state.move_down();
        assert_eq!(state.note_selected, 1);
        state.switch_tab();
        assert_eq!(state.tab, WorkspaceTab::Todos);
        state.move_down();
        assert_eq!(state.todo_selected, 0);
        assert_eq!(state.selected_todo().unwrap().title, "todo");
    }
}
