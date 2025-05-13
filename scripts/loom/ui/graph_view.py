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
from collections import deque

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
    def axial_to_pixel(q, r, radius):
        """Convert axial hex coordinates (q, r) to pixel (x, y)."""
        x = radius * 3/2 * q
        y = radius * math.sqrt(3) * (r + q / 2)
        return x, y

    def draw_graph(self, notes, links):
        radius = NODE_RADIUS
        visited = set()
        positions = {}
        used_coords = set()
        print("Links:", links)
        print("Notes:", list(notes.keys())[:10])
        # Build adjacency list from links
        adjacency = {note_id: set() for note_id in notes}
        for source_id, target_id in links:
            if source_id in notes and target_id in notes:
                adjacency[source_id].add(target_id)
                adjacency[target_id].add(source_id)

        for node, neighbors in adjacency.items():
            print(f"  {node}: {list(neighbors)}")
        # Pick an arbitrary start node
        if not notes:
            return
        start_id = next(iter(notes))
        queue = deque([(start_id, 0, 0)])
        used_coords.add((0, 0))

        while queue:
            note_id, q, r = queue.popleft()

            if note_id in visited:
                continue
            visited.add(note_id)

            x, y = self.axial_to_pixel(q, r, radius)
            print(f"Placing node {note_id} at axial ({q}, {r}) → pixel ({x:.1f}, {y:.1f})")

            ellipse = QGraphicsEllipseItem(0, 0, radius * 2, radius * 2)
            ellipse.setBrush(QColor("skyblue"))
            ellipse.setPen(QPen(Qt.GlobalColor.black))
            ellipse.setPos(x, y)
            ellipse.setData(0, notes[note_id]["path"])
            ellipse.setToolTip(notes[note_id]["title"])
            ellipse.setFlag(
                QGraphicsEllipseItem.GraphicsItemFlag.ItemIsSelectable)
            ellipse.setFlag(
                QGraphicsEllipseItem.GraphicsItemFlag.ItemIsMovable)

            self.scene.addItem(ellipse)
            self.nodes[note_id] = ellipse
            positions[note_id] = QPointF(x + radius, y + radius)

            # Assign axial coordinates to unvisited neighbors
            neighbors = list(adjacency[note_id])
            for direction, neighbor_id in zip(HEX_DIRECTIONS, neighbors):
                if neighbor_id in visited:
                    continue

                dq, dr = direction
                new_q, new_r = q + dq, r + dr

                # Find an unused position
                while (new_q, new_r) in used_coords:
                    new_q += dq
                    new_r += dr

                used_coords.add((new_q, new_r))
                queue.append((neighbor_id, new_q, new_r))

        # Draw edges
        pen = QPen(Qt.GlobalColor.darkGray)
        for source_id, target_path in links:
            source_ellipse = self.nodes.get(source_id)
            if not source_ellipse:
                continue
            for target_id, note in notes.items():
                if note["path"] == target_path and target_id in self.nodes:
                    src_center = positions[source_id]
                    dst_center = positions[target_id]
                    line = QGraphicsLineItem(
                        src_center.x(),
                        src_center.y(),
                        dst_center.x(),
                        dst_center.y()
                    )
                    line.setPen(pen)
                    self.scene.addItem(line)
        self.setSceneRect(self.scene.itemsBoundingRect())
        self.fitInView(self.scene.sceneRect(),
                       Qt.AspectRatioMode.KeepAspectRatio)
