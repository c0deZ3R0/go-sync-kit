package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Categories []Category `yaml:"categories"`
}

type Category struct {
	Key   string `yaml:"key"`
	Title string `yaml:"title"`
	Items []Item `yaml:"items"`
}

type Item struct {
	ID     string   `yaml:"id"`
	Name   string   `yaml:"name"`
	Status string   `yaml:"status"` // stable|experimental
	Docs   []string `yaml:"docs"`
	Notes  string   `yaml:"notes,omitempty"`
	Match  string   `yaml:"match,omitempty"` // optional substring to look for in docs
}

type mdRow struct {
	Category string
	Name     string
	Status   string
}

func main() {
	yamlPath := flag.String("yaml", "docs/status.yaml", "path to status.yaml")
	mdPath := flag.String("md", "docs/status.md", "path to status.md")
	warnUnknown := flag.Bool("warn-unknown", true, "warn on features in docs not present in YAML")
	flag.Parse()

	cfg, err := loadYAML(*yamlPath)
	if err != nil {
		fail(err)
	}

	mdBytes, err := os.ReadFile(*mdPath)
	if err != nil {
		fail(fmt.Errorf("read %s: %w", *mdPath, err))
	}
	md := string(mdBytes)

	var errs []string
	var warns []string

	// basic structure checks + file existence + presence in md (name or match)
	seenIDs := make(map[string]struct{})
	for _, cat := range cfg.Categories {
		if strings.TrimSpace(cat.Key) == "" || strings.TrimSpace(cat.Title) == "" {
			errs = append(errs, fmt.Sprintf("category missing key and/or title: %+v", cat))
		}
		for _, it := range cat.Items {
			scope := fmt.Sprintf("%s/%s", cat.Key, it.ID)
			if it.ID == "" {
				errs = append(errs, fmt.Sprintf("[%s] missing id", scope))
			}
			if _, dup := seenIDs[it.ID]; dup {
				errs = append(errs, fmt.Sprintf("[%s] duplicate id %q", scope, it.ID))
			}
			seenIDs[it.ID] = struct{}{}

			if it.Name == "" {
				errs = append(errs, fmt.Sprintf("[%s] missing name", scope))
			}
			status := normStatus(it.Status)
			if status != "stable" && status != "experimental" {
				errs = append(errs, fmt.Sprintf("[%s] invalid status %q (want stable|experimental)", scope, it.Status))
			}
			if len(it.Docs) == 0 {
				errs = append(errs, fmt.Sprintf("[%s] no docs[] listed", scope))
			}
			for _, p := range it.Docs {
				if _, err := os.Stat(filepath.Clean(p)); err != nil {
					errs = append(errs, fmt.Sprintf("[%s] doc path not found: %s", scope, p))
				}
			}
			needle := it.Name
			if strings.TrimSpace(it.Match) != "" {
				needle = it.Match
			}
			if !containsCaseFold(md, needle) {
				errs = append(errs, fmt.Sprintf("[%s] %q not found in %s (consider setting match:)", scope, needle, *mdPath))
			}
		}
	}

	// optional: parse markdown tables and compare displayed status/category
	rows := parseMarkdownTables(md)
	mdIndex := make(map[string]mdRow)
	for _, r := range rows {
		if r.Name == "" {
			continue
		}
		mdIndex[strings.ToLower(r.Name)] = r
	}

	for _, cat := range cfg.Categories {
		for _, it := range cat.Items {
			key := strings.ToLower(it.Name)
			if strings.TrimSpace(it.Match) != "" {
				key = strings.ToLower(it.Match)
			}
			if r, ok := mdIndex[key]; ok {
				if !eqStatus(it.Status, r.Status) {
					warns = append(warns, fmt.Sprintf("[%s/%s] YAML status %q != displayed %q in docs", cat.Key, it.ID, it.Status, r.Status))
				}
				// category mismatch is a warning only; headings/labels can vary
				if r.Category != "" && !strings.EqualFold(r.Category, cat.Title) && !strings.EqualFold(r.Category, cat.Key) {
					warns = append(warns, fmt.Sprintf("[%s/%s] YAML category %q differs from docs table %q", cat.Key, it.ID, cat.Title, r.Category))
				}
			}
		}
	}

	if *warnUnknown {
		for _, r := range rows {
			if !yamlHasName(cfg, strings.ToLower(r.Name)) {
				warns = append(warns, fmt.Sprintf("[docs-only] %q in %s (category=%q, status=%q) not present in YAML", r.Name, *mdPath, r.Category, r.Status))
			}
		}
	}

	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "WARN: %s\n", w)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", e)
		}
		os.Exit(1)
	}
	fmt.Println("status-check: OK")
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
	os.Exit(1)
}

func loadYAML(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

func containsCaseFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func normStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(s, "stab"):
		return "stable"
	case strings.HasPrefix(s, "exp"):
		return "experimental"
	default:
		return s
	}
}
func eqStatus(a, b string) bool { return normStatus(a) == normStatus(b) }

func yamlHasName(cfg *Config, nameLower string) bool {
	for _, c := range cfg.Categories {
		for _, it := range c.Items {
			n := strings.ToLower(it.Name)
			if strings.TrimSpace(it.Match) != "" {
				n = strings.ToLower(it.Match)
			}
			if n == nameLower {
				return true
			}
		}
	}
	return false
}

func parseMarkdownTables(md string) []mdRow {
	lines := strings.Split(md, "\n")
	var out []mdRow
	var currentCategory string

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Track category headings
		if strings.HasPrefix(line, "## ") {
			currentCategory = strings.TrimSpace(strings.TrimPrefix(line, "##"))
		}

		if looksLikeHeaderRow(line) && i+1 < len(lines) && isSeparatorLine(lines[i+1]) {
			headers := splitRow(line)
			// parse following body rows
			for j := i + 2; j < len(lines); j++ {
				row := strings.TrimSpace(lines[j])
				if !looksLikeBodyRow(row) {
					break
				}
				cols := splitRow(row)
				r := mdRow{
					Category: currentCategory,
					Name:     stripMarkdown(getCol(headers, cols, []string{"feature", "component", "capability", "name"})),
					Status:   normStatus(getCol(headers, cols, []string{"status"})),
				}
				if r.Name != "" {
					out = append(out, r)
				}
				i = j
			}
		}
	}
	return out
}

func stripMarkdown(s string) string {
	// Remove markdown bold **text**
	s = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(s, "$1")
	// Remove markdown links [text](url)
	s = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

func looksLikeHeaderRow(s string) bool {
	return strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|") && strings.Count(s, "|") >= 3
}
func looksLikeBodyRow(s string) bool {
	return strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|") && !isSeparatorLine(s)
}
func isSeparatorLine(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "|") || !strings.HasSuffix(s, "|") {
		return false
	}
	// normalize: remove pipes and spaces, should be dashes and colons
	core := strings.Trim(strings.ReplaceAll(s, " ", ""), "|")
	re := regexp.MustCompile(`^-{3,}(:)?(\|:{0,1}-+:?)*$`)
	return re.MatchString(core)
}
func splitRow(s string) []string {
	parts := strings.Split(s, "|")
	var out []string
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	// drop leading/trailing empties from table syntax
	if len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}
func getCol(headers, cols []string, wanted []string) string {
	idx := -1
	for i, h := range headers {
		lh := strings.ToLower(strings.TrimSpace(h))
		for _, w := range wanted {
			if lh == w {
				idx = i
				break
			}
		}
		if idx != -1 {
			break
		}
	}
	if idx >= 0 && idx < len(cols) {
		return strings.TrimSpace(cols[idx])
	}
	return ""
}
