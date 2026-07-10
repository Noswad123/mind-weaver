package view

type View struct {
  UID     string         `json:"uid"`
  Path    string         `json:"path"`
  Title   string         `json:"title"`
  Domain  string         `json:"domain"`
  Meta    map[string]any `json:"meta"`
  Sections map[string]any `json:"sections"`
}
