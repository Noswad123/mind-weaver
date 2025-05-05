import os
import math
import sys
import sqlite3
from PyQt6.QtWidgets import QApplication, QMainWindow, QGraphicsView, QGraphicsScene, QGraphicsEllipseItem, QGraphicsLineItem, QTextEdit, QHBoxLayout, QWidget, QSplitter, QPushButton, QVBoxLayout
from PyQt6.QtGui import QPen, QColor, QPainter
from PyQt6.QtCore import Qt, QPointF
from dotenv import load_dotenv

load_dotenv()

DB_PATH = os.getenv("DB_PATH", "./")
NOTES_DIR = os.getenv("NOTES_DIR", "./")
NODE_RADIUS = 10

class GraphView(QGraphicsView):
    def __init__(self, notes, links, content_panel):
        super().__init__()
        self.scene = QGraphicsScene(self)
        self.setScene(self.scene)
        self.nodes = {}  # id -> ellipse
        self.content_panel = content_panel

        self.draw_graph(notes, links)
        self.setRenderHint(QPainter.RenderHint.Antialiasing)
        self.setDragMode(QGraphicsView.DragMode.ScrollHandDrag)
        self.scale(1.2, 1.2)

    def wheelEvent(self, event):
        zoomInFactor = 1.15
        zoomOutFactor = 1 / zoomInFactor

        if event.angleDelta().y() > 0:
            self.scale(zoomInFactor, zoomInFactor)
        else:
            self.scale(zoomOutFactor, zoomOutFactor)

    def draw_graph(self, notes, links):
        radius = NODE_RADIUS
        x_spacing = radius * 2 * 0.87  # horizontal distance between centers
        y_spacing = radius * 2 * 0.75  # vertical step, staggered rows

        positions = {}  # note_id -> QPointF center
        row = 0
        col = 0

        for i, (note_id, note) in enumerate(notes.items()):
            # stagger y every other column for hex grid
            x = col * x_spacing
            y = row * y_spacing + (col % 2) * (y_spacing / 2)

            ellipse = QGraphicsEllipseItem(0, 0, radius * 2, radius * 2)
            ellipse.setBrush(QColor("skyblue"))
            ellipse.setPen(QPen(Qt.GlobalColor.black))
            ellipse.setPos(x, y)
            ellipse.setData(0, note["path"])
            ellipse.setToolTip(note["title"])
            ellipse.setFlag(QGraphicsEllipseItem.GraphicsItemFlag.ItemIsSelectable)
            ellipse.setFlag(QGraphicsEllipseItem.GraphicsItemFlag.ItemIsMovable)

            self.scene.addItem(ellipse)
            self.nodes[note_id] = ellipse
            positions[note_id] = QPointF(x + radius, y + radius)  # store center

            row += 1
            if row > 5:
                row = 0
                col += 1

        pen = QPen(Qt.GlobalColor.darkGray)
        for source_id, target_path in links:
            source_ellipse = self.nodes.get(source_id)
            if not source_ellipse:
                continue
            for target_id, target_note in notes.items():
                if target_note["path"] == target_path and target_id in self.nodes:
                    src_center = positions[source_id]
                    dst_center = positions[target_id]
                    line = QGraphicsLineItem(src_center.x(), src_center.y(), dst_center.x(), dst_center.y())
                    line.setPen(pen)
                    self.scene.addItem(line)

    def mousePressEvent(self, event):
        super().mousePressEvent(event)
        item = self.itemAt(event.pos())
        if isinstance(item, QGraphicsEllipseItem):
            note_rel_path = item.data(0)
            notes_root = NOTES_DIR
            full_path = os.path.join(notes_root, note_rel_path)

            try:
                with open(full_path, "r") as f:
                    self.content_panel.setPlainText(f"# {full_path}\n\n" + f.read())
            except Exception as e:
                self.content_panel.setPlainText(f"Error loading {full_path}: {e}")

class MainWindow(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("Mind Weaver (PyQt)")
        self.resize(1200, 800)

        notes, links = self.load_data()

        # Splitter for graph + text panel
        splitter = QSplitter()

        # 1. GraphView
        self.content_panel = QTextEdit()
        self.content_panel.setReadOnly(True)
        graph_view = GraphView(notes, links, self.content_panel)
        splitter.addWidget(graph_view)

        # 2. Toggle button + content panel
        toggle_button = QPushButton("Hide Preview")
        toggle_button.setCheckable(True)

        def toggle_panel():
            visible = not self.content_panel.isVisible()
            self.content_panel.setVisible(visible)
            toggle_button.setText("Show Preview" if not visible else "Hide Preview")

        toggle_button.clicked.connect(toggle_panel)

        text_panel = QWidget()
        text_layout = QVBoxLayout()
        text_layout.addWidget(toggle_button)
        text_layout.addWidget(self.content_panel)
        text_panel.setLayout(text_layout)
        text_panel.setMinimumWidth(300)
        text_panel.setMaximumWidth(300)
        splitter.addWidget(text_panel)

        splitter.setSizes([900, 300])  # default sizes

        # Final layout
        layout = QHBoxLayout()
        layout.addWidget(splitter)

        container = QWidget()
        container.setLayout(layout)
        self.setCentralWidget(container)

    def load_data(self):
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        notes = {
            row['id']: {'path': row['path'], 'title': row['title'] or row['path']}
            for row in conn.execute("SELECT id, path, title FROM notes")
        }
        links = [
            (row['note_id'], row['resolved_path'])
            for row in conn.execute("SELECT note_id, resolved_path FROM links WHERE type='internal'")
        ]
        return notes, links

if __name__ == '__main__':
    app = QApplication(sys.argv)
    window = MainWindow()
    window.show()
    sys.exit(app.exec())
