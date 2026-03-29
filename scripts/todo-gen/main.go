package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var todoPattern = regexp.MustCompile(`TODO\(([^)]+)\):\s*(.+)$`)

type todoItem struct {
	Owner string
	Text  string
	Path  string
	Line  int
}

func main() {
	rootFlag := flag.String("root", "..", "repository root path")
	outFlag := flag.String("out", "../docs/exec-plans/TODO.md", "output markdown file path")
	flag.Parse()

	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fatalf("resolve root path: %v", err)
	}
	outPath, err := filepath.Abs(*outFlag)
	if err != nil {
		fatalf("resolve output path: %v", err)
	}

	items, err := collectTODOItems(root)
	if err != nil {
		fatalf("collect TODO comments: %v", err)
	}

	if err := writeMarkdown(root, outPath, items); err != nil {
		fatalf("write output file: %v", err)
	}

	fmt.Printf("Generated %s with %d TODO items\n", outPath, len(items))
}

func collectTODOItems(root string) ([]todoItem, error) {
	items := make([]todoItem, 0)

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if shouldSkipDir(path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipFile(path) {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			idx := strings.Index(line, "TODO(")
			if idx < 0 || !isLikelyCommentLine(line, idx) {
				continue
			}

			matches := todoPattern.FindStringSubmatch(line[idx:])
			if len(matches) != 3 {
				continue
			}

			owner := strings.TrimSpace(matches[1])
			text := strings.TrimSpace(matches[2])
			text = strings.TrimSuffix(text, "*/")
			text = strings.TrimSpace(text)
			if owner == "" || text == "" {
				continue
			}

			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)

			items = append(items, todoItem{
				Owner: owner,
				Text:  text,
				Path:  relPath,
				Line:  lineNo,
			})
		}

		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Line < items[j].Line
		}
		return items[i].Path < items[j].Path
	})

	return items, nil
}

func shouldSkipDir(path, name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".idea", ".vscode", "bin", "dist":
		return true
	}

	normalized := filepath.ToSlash(path)
	if strings.Contains(normalized, "/backend/bin") || strings.Contains(normalized, "/frontend/dist") {
		return true
	}

	return false
}

func shouldSkipFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && base != ".golangci.yml" {
		return true
	}
	if strings.HasSuffix(base, ".md") {
		return true
	}
	if strings.HasSuffix(base, ".png") || strings.HasSuffix(base, ".jpg") || strings.HasSuffix(base, ".jpeg") || strings.HasSuffix(base, ".gif") || strings.HasSuffix(base, ".webp") || strings.HasSuffix(base, ".svg") {
		return true
	}
	if strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".exe") || strings.HasSuffix(base, ".dll") || strings.HasSuffix(base, ".so") {
		return true
	}
	return false
}

func isLikelyCommentLine(line string, todoIndex int) bool {
	prefix := line[:todoIndex]
	trimmed := strings.TrimSpace(prefix)
	if strings.Contains(prefix, "//") || strings.Contains(prefix, "#") || strings.Contains(prefix, "/*") || strings.Contains(prefix, "<!--") {
		return true
	}
	return strings.HasPrefix(trimmed, "*")
}

func writeMarkdown(root, outPath string, items []todoItem) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	now := time.Now().Format(time.RFC3339)
	if _, err := fmt.Fprintln(f, "# TODO Index"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "\nGenerated at: %s\n", now); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, "Source pattern: TODO(username): description"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "Repository root: %s\n", filepath.ToSlash(root)); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(f, "\n## Summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "- Total TODO items: %d\n", len(items)); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(f, "\n## Items"); err != nil {
		return err
	}

	if len(items) == 0 {
		if _, err := fmt.Fprintln(f, "- No TODO items found."); err != nil {
			return err
		}
		return nil
	}

	for _, item := range items {
		if _, err := fmt.Fprintf(f, "- [ ] %s: %s (%s:%d)\n", item.Owner, item.Text, item.Path, item.Line); err != nil {
			return err
		}
	}

	return nil
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
