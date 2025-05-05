from PyQt6.QtWidgets import QGraphicsView, QGraphicsScene, QGraphicsEllipseItem, QGraphicsLineItem
from PyQt6.QtGui import QPen, QColor, QPainter
from PyQt6.QtCore import Qt, QPointF
from config import NODE_RADIUS
from utils.file_loader import FileLoader

class GraphView(QGraphicsView):
    def __init__(self, notes, links, content_panel):
        super().__init__()
        self.scene = QGraphicsScene(self)
        self.setScene(self.scene)
        self.nodes = {}
        self.content_panel = content_panel
        self.file_loader = FileLoader()

        self.draw_graph(notes, links)
        self.setRenderHint(QPainter.RenderHint.Antialiasing)
        self.setDragMode(QGraphicsView.DragMode.ScrollHandDrag)
        self.scale(1.2, 1.2)

    def wheelEvent(self, event):
        zoomInFactor = 1.15
        zoomOutFactor = 1 / zoomInFactor
        self.scale(zoomInFactor, zoomInFactor) if event.angleDelta().y() > 0 else self.scale(zoomOutFactor, zoomOutFactor)

    def draw_graph(self, notes, links):
        radius = NODE_RADIUS
        x_spacing = radius * 2 * 0.87
        y_spacing = radius * 2 * 0.75

        positions = {}
        row, col = 0, 0

        for note_id, note in notes.items():
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
            positions[note_id] = QPointF(x + radius, y + radius)

            row += 1
            if row > 5:
                row = 0
                col += 1

        pen = QPen(Qt.GlobalColor.darkGray)
        for source_id, target_path in links:
            src = self.nodes.get(source_id)
            if not src:
                continue
            for target_id, note in notes.items():
                if note["path"] == target_path and target_id in self.nodes:
                    src_center = positions[source_id]
                    dst_center = positions[target_id]
                    line = QGraphicsLineItem(src_center.x(), src_center.y(), dst_center.x(), dst_center.y())
                    line.setPen(pen)
                    self.scene.addItem(line)

    def mousePressEvent(self, event):
        super().mousePressEvent(event)
        item = self.itemAt(event.pos())
        if isinstance(item, QGraphicsEllipseItem):
            content = self.file_loader.load_note_content(item.data(0))
            self.content_panel.setPlainText(content)
