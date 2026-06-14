export type StatusMessage = {
  kind: 'idle' | 'ok' | 'error'
  text: string
}

export type AppTab = 'notes' | 'todos' | 'sync' | 'settings'

export type NoteMode = 'read' | 'edit'
