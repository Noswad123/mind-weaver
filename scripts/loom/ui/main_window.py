from PyQt6.QtWidgets import QMainWindow, QTextEdit, QSplitter, QPushButton, QWidget, QVBoxLayout, QHBoxLayout
from models.note_repository import NoteRepository
from ui.graph_view import GraphView

class MainWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("Mind Weaver (PyQt)")
        self.resize(1200, 800)

        repo = NoteRepository()
        notes = repo.load_notes()
        links = repo.load_links()

        self.content_panel = QTextEdit()
        self.content_panel.setReadOnly(True)

        graph_view = GraphView(notes, links, self.content_panel)
        toggle_button = QPushButton("Hide Preview")
        toggle_button.setCheckable(True)
        toggle_button.clicked.connect(lambda: self.toggle_panel(toggle_button))

        text_panel = QWidget()
        text_layout = QVBoxLayout()
        text_layout.addWidget(toggle_button)
        text_layout.addWidget(self.content_panel)
        text_panel.setLayout(text_layout)
        text_panel.setMinimumWidth(300)
        text_panel.setMaximumWidth(300)

        splitter = QSplitter()
        splitter.addWidget(graph_view)
        splitter.addWidget(text_panel)
        splitter.setSizes([900, 300])

        layout = QHBoxLayout()
        layout.addWidget(splitter)
        container = QWidget()
        container.setLayout(layout)
        self.setCentralWidget(container)

    def toggle_panel(self, button):
        visible = not self.content_panel.isVisible()
        self.content_panel.setVisible(visible)
        button.setText("Show Preview" if not visible else "Hide Preview")
