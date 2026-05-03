package model

// SystemLogEntry represents one backend system log line exposed to the UI.
type SystemLogEntry struct {
	Time       string         `json:"time"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Raw        string         `json:"raw,omitempty"`
}

// SystemLogsResponse contains a filtered slice of backend system logs.
type SystemLogsResponse struct {
	Path    string           `json:"path"`
	Entries []SystemLogEntry `json:"entries"`
}
