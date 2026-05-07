package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"minedock/backend/internal/model"
)

const (
	defaultSystemLogPath = "data/minedock.log"
	maxSystemLogTail     = 5000
)

// SystemLogSink owns the backend log file used by the default slog logger.
type SystemLogSink struct {
	path string
	file *os.File
}

// NewSystemLogSink configures slog to write JSON logs to stdout and a file.
func NewSystemLogSink(path string) (*SystemLogSink, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultSystemLogPath
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	writer := io.MultiWriter(os.Stdout, file)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	return &SystemLogSink{path: path, file: file}, nil
}

// Path returns the configured log file path.
func (s *SystemLogSink) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Close flushes and closes the log file.
func (s *SystemLogSink) Close() error {
	if s == nil || s.file == nil {
		return nil
	}
	return s.file.Close()
}

// SystemLogQuery defines filters for reading backend system logs.
type SystemLogQuery struct {
	Tail  int
	Level string
	Query string
}

// SystemLogService reads backend system logs from the configured file.
type SystemLogService struct {
	path string
}

// NewSystemLogService creates a SystemLogService.
func NewSystemLogService(path string) *SystemLogService {
	return &SystemLogService{path: strings.TrimSpace(path)}
}

// List returns filtered system log entries.
func (s *SystemLogService) List(ctx context.Context, query SystemLogQuery) (*model.SystemLogsResponse, error) {
	path := strings.TrimSpace(s.path)
	if path == "" {
		path = defaultSystemLogPath
	}

	tail := query.Tail
	if tail <= 0 {
		tail = 500
	}
	if tail > maxSystemLogTail {
		tail = maxSystemLogTail
	}

	levelStr := strings.ToLower(strings.TrimSpace(query.Level))
	levelMap := make(map[string]bool)
	if levelStr != "" {
		for _, l := range strings.Split(levelStr, ",") {
			levelMap[strings.TrimSpace(l)] = true
		}
	}

	textQuery := strings.ToLower(strings.TrimSpace(query.Query))

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.SystemLogsResponse{Path: path, Entries: []model.SystemLogEntry{}}, nil
		}
		return nil, fmt.Errorf("open system log: %w", err)
	}
	defer file.Close()

	entries := make([]model.SystemLogEntry, 0, tail)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		raw := scanner.Text()
		if textQuery != "" && !strings.Contains(strings.ToLower(raw), textQuery) {
			continue
		}

		entry := parseSystemLogLine(raw)
		if len(levelMap) > 0 && !levelMap[strings.ToLower(entry.Level)] {
			continue
		}

		if len(entries) == tail {
			copy(entries, entries[1:])
			entries[len(entries)-1] = entry
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan system log: %w", err)
	}

	return &model.SystemLogsResponse{Path: path, Entries: entries}, nil
}

func parseSystemLogLine(raw string) model.SystemLogEntry {
	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return model.SystemLogEntry{Level: "unknown", Message: raw, Raw: raw}
	}

	entry := model.SystemLogEntry{
		Level:      stringField(fields, "level", "unknown"),
		Message:    stringField(fields, "msg", ""),
		Time:       stringField(fields, "time", ""),
		Attributes: map[string]any{},
		Raw:        raw,
	}
	for key, value := range fields {
		switch key {
		case "time", "level", "msg":
			continue
		default:
			entry.Attributes[key] = value
		}
	}
	if len(entry.Attributes) == 0 {
		entry.Attributes = nil
	}
	return entry
}

func stringField(fields map[string]any, key string, fallback string) string {
	value, ok := fields[key]
	if !ok {
		return fallback
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fallback
}
