export type StatusMessage = {
  kind: 'idle' | 'ok' | 'error'
  text: string
}

export type AppTab = 'notes' | 'todos' | 'sync' | 'conflicts' | 'settings'

export type NoteMode = 'read' | 'edit'
