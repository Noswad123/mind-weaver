from PyQt6.QtWidgets import (
    QGraphicsView,
    QGraphicsScene,
    QGraphicsEllipseItem,
    QGraphicsLineItem
)
from PyQt6.QtGui import QPen, QColor, QPainter
from PyQt6.QtCore import Qt, QPointF
from config import NODE_RADIUS
from utils.file_loader import FileLoader
import math

HEX_DIRECTIONS = [(+1, 0), (+1, -1), (0, -1), (-1, 0), (-1, +1), (0, +1)]


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

    def _normalize_links(self, notes, links):
        """Convert (source_id, target_path) → (source_id, target_id)"""
        path_to_id = {note["path"]: note_id for note_id, note in notes.items()}
        normalized = []

        for source_id, target_path in links:
            target_id = path_to_id.get(target_path)
            if target_id:
                normalized.append((source_id, target_id))
        return normalized

    def wheelEvent(self, event):
        zoomInFactor = 1.15
        zoomOutFactor = 1 / zoomInFactor
        self.scale(zoomInFactor, zoomInFactor) if event.angleDelta(
        ).y() > 0 else self.scale(zoomOutFactor, zoomOutFactor)

    def mousePressEvent(self, event):
        super().mousePressEvent(event)
        item = self.itemAt(event.pos())
        if isinstance(item, QGraphicsEllipseItem):
            content = self.file_loader.load_note_content(item.data(0))
            self.content_panel.setPlainText(content)

    @staticmethod
    def axial_to_pixel(q, r, spacing):
        """Convert axial hex coordinates (q, r) to pixel (x, y)."""
        x = spacing * 3/2 * q
        y = spacing * math.sqrt(3) * (r + q / 2)
        return x, y

    def draw_graph(self, notes, links):
        if not notes:
            return

        links = self._normalize_links(notes, links)
        adjacency = self._build_adjacency_list(notes, links)
        positions = self._place_nodes(notes)
        self._draw_edges(links, positions)
        self._finalize_scene()

    def _build_adjacency_list(self, notes, links):
        adjacency = {note_id: set() for note_id in notes}
        for source_id, target_id in links:
            if source_id in notes and target_id in notes:
                adjacency[source_id].add(target_id)
                adjacency[target_id].add(source_id)
        return adjacency

    def _place_nodes(self, notes):
        radius = NODE_RADIUS
        positions = {}
        used_coords = set()

        # Define grid size
        grid_width = int(len(notes) ** 0.5) + 1  # for a roughly square layout
        grid_height = (len(notes) // grid_width) + 1

        note_ids = list(notes.keys())
        index = 0

        for r in range(grid_height):
            for q in range(grid_width):
                if index >= len(note_ids):
                    break

                # offset every other row for hex layout (pointy-topped)
                offset = r // 2
                axial_q = q - offset
                axial_r = r

                if (axial_q, axial_r) in used_coords:
                    continue

                spacing = NODE_RADIUS * 2.5
                x, y = self.axial_to_pixel(axial_q, axial_r, spacing)
                note_id = note_ids[index]
                ellipse = self._create_node_ellipse(notes[note_id], x, y, radius)
                self.scene.addItem(ellipse)
                self.nodes[note_id] = ellipse
                positions[note_id] = QPointF(x + radius, y + radius)
                used_coords.add((axial_q, axial_r))
                index += 1

        return positions

    def _create_node_ellipse(self, note, x, y, radius):
        ellipse = QGraphicsEllipseItem(0, 0, radius * 2, radius * 2)
        ellipse.setBrush(QColor("skyblue"))
        ellipse.setPen(QPen(Qt.GlobalColor.black))
        ellipse.setPos(x, y)
        ellipse.setData(0, note["path"])
        ellipse.setToolTip(note["title"])
        ellipse.setFlag(QGraphicsEllipseItem.GraphicsItemFlag.ItemIsSelectable)
        ellipse.setFlag(QGraphicsEllipseItem.GraphicsItemFlag.ItemIsMovable)

        return ellipse

    def _draw_edges(self, links, positions):
        pen = QPen(Qt.GlobalColor.darkGray)

        for source_id, target_id in links:
            if source_id not in self.nodes or target_id not in self.nodes:
                continue

            src_center = positions[source_id]
            dst_center = positions[target_id]
            line = QGraphicsLineItem(
                src_center.x(), src_center.y(),
                dst_center.x(), dst_center.y()
            )
            line.setPen(pen)
            self.scene.addItem(line)

    def _finalize_scene(self):
        self.setSceneRect(self.scene.itemsBoundingRect())
        self.fitInView(
            self.scene.sceneRect(),
            Qt.AspectRatioMode.KeepAspectRatio
        )
