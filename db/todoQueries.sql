SELECT
  todos.id,
  todos.task,
  todos.status,
  todos.raw_status,
  todos.depth,
  todos.line_number,
  task_groups.name AS group_name,
  task_groups.level AS group_level,
  notes.path AS note_path
FROM todos
JOIN task_groups ON todos.task_group_id = task_groups.id
JOIN notes ON todos.note_path = notes.path
ORDER BY notes.path, task_groups.level, todos.depth, todos.line_number;


SELECT
  todos.task,
  todos.status,
  task_groups.name AS group_name,
  notes.title
FROM todos
JOIN task_groups ON todos.task_group_id = task_groups.id
JOIN notes ON todos.note_path = notes.path
WHERE task_groups.name = 'Afterwork checklist';

SELECT
  g1.name AS group_name,
  g1.level,
  g2.name AS derived_from
FROM task_groups g1
LEFT JOIN task_groups g2 ON g1.derived_group_id = g2.id
ORDER BY g1.note_path, g1.line_number;

WITH RECURSIVE all_subgroups(parent_id, child_id) AS (
  SELECT parent_id, child_id FROM group_subgroups WHERE parent_id = 42
  UNION
  SELECT g.parent_id, gs.child_id
  FROM group_subgroups gs
  JOIN all_subgroups g ON gs.parent_id = g.child_id
)
SELECT
  todos.task,
  todos.status,
  task_groups.name AS group_name
FROM todos
JOIN task_groups ON todos.task_group_id = task_groups.id
WHERE task_group_id IN (
  SELECT child_id FROM all_subgroups
  UNION SELECT 42
);
