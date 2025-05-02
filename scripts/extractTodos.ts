interface ParsedTodo {
  group: string;               // immediate group name (e.g., "** Afterwork checklist")
  derivedGroup?: string;       // higher-level group this one came from
  task?: string;               // actual task, undefined for group headings
  status: string;              // "done", "todo", etc.
  rawStatus: string;           // raw symbol (e.g., "x", "!")
  level: number;               // number of asterisks in heading
  depth: number;               // number of dashes before the task
  line: number;                // line number in file
  isGroup: boolean;
}

const statusMap: Record<string, string> = {
  'x': 'done',
  '=': 'hold',
  '?': 'ambiguous',
  ' ': 'todo',
  '-': 'pending',
  '+': 'recurring',
  '_': 'cancelled',
  '!': 'important'
};

function extractTodos(content: string): ParsedTodo[] {
  const lines = content.split('\n');
  const todos: ParsedTodo[] = [];
  const groupStack: { level: number; name: string }[] = [];

  lines.forEach((line, index) => {
    const lineNum = index + 1;

    // Match group headings like: ** (!) Group Name
    const groupMatch = line.match(/^(\*+)\s+(?:\((.)\)\s+)?(.*)$/);
    if (groupMatch) {
      const level = groupMatch[1].length;
      const rawStatus = groupMatch[2] ?? ' ';
      const name = groupMatch[3].trim();
      const status = statusMap[rawStatus] ?? 'todo';

      // Maintain group stack for derivedGroup tracking
      while (groupStack.length > 0 && groupStack[groupStack.length - 1].level >= level) {
        groupStack.pop();
      }

      groupStack.push({ level, name });

      todos.push({
        group: name,
        derivedGroup: groupStack.length >= 2 ? groupStack[groupStack.length - 2].name : undefined,
        task: name,
        status,
        rawStatus,
        level,
        depth: 0,
        line: lineNum,
        isGroup: true
      });

      return;
    }

    // Match task lines like: - (x) Take out trash
    const taskMatch = line.match(/^(\s*)(-+)\s+\((.)\)\s+(.*)$/);
    if (taskMatch) {
      const [, , dashes, rawStatus, task] = taskMatch;
      const depth = dashes.length;
      const status = statusMap[rawStatus] ?? 'todo';

      const currentGroup = groupStack[groupStack.length - 1]?.name ?? '';
      const currentLevel = groupStack[groupStack.length - 1]?.level ?? 0;
      const derivedGroup = groupStack.length >= 2 ? groupStack[groupStack.length - 2].name : undefined;

      todos.push({
        group: currentGroup,
        derivedGroup,
        task: task.trim(),
        status,
        rawStatus,
        level: currentLevel,
        depth,
        line: lineNum,
        isGroup: false
      });
    }
  });

  return todos;
}

export {extractTodos, type ParsedTodo};
