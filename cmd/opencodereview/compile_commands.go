package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type compileCommandEntry struct {
	Directory string   `json:"directory"`
	Command   string   `json:"command,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
	File      string   `json:"file"`
}

type rulesCompileCommandOptions struct {
	Files        []string
	IncludePaths []string
	Defines      []string
	Undefines    []string
	Standard     string
}

func loadRulesCompileCommands(path string) (rulesCompileCommandOptions, error) {
	if path == "" {
		return rulesCompileCommandOptions{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rulesCompileCommandOptions{}, fmt.Errorf("read compile commands %s: %w", path, err)
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var entries []compileCommandEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return rulesCompileCommandOptions{}, fmt.Errorf("unmarshal compile commands %s: %w", path, err)
	}

	var out rulesCompileCommandOptions
	for _, entry := range entries {
		directory := entry.Directory
		if directory == "" {
			directory = filepath.Dir(path)
		}
		if entry.File != "" {
			out.Files = append(out.Files, resolveCompileCommandPath(directory, entry.File))
		}
		args := entry.Arguments
		if len(args) == 0 && entry.Command != "" {
			args = splitCompileCommandLine(entry.Command)
		}
		updateCompileCommandOptions(&out, directory, args)
	}
	out.Files = dedupeStrings(out.Files)
	out.IncludePaths = dedupeStrings(out.IncludePaths)
	out.Defines = dedupeStrings(out.Defines)
	out.Undefines = dedupeStrings(out.Undefines)
	return out, nil
}

func updateCompileCommandOptions(out *rulesCompileCommandOptions, directory string, args []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-I" || arg == "/I":
			if i+1 < len(args) {
				i++
				out.IncludePaths = append(out.IncludePaths, resolveCompileCommandPath(directory, args[i]))
			}
		case strings.HasPrefix(arg, "-I") && len(arg) > 2:
			out.IncludePaths = append(out.IncludePaths, resolveCompileCommandPath(directory, arg[2:]))
		case strings.HasPrefix(arg, "/I") && len(arg) > 2:
			out.IncludePaths = append(out.IncludePaths, resolveCompileCommandPath(directory, arg[2:]))
		case arg == "-isystem":
			if i+1 < len(args) {
				i++
				out.IncludePaths = append(out.IncludePaths, resolveCompileCommandPath(directory, args[i]))
			}
		case arg == "-D" || arg == "/D":
			if i+1 < len(args) {
				i++
				out.Defines = append(out.Defines, args[i])
			}
		case strings.HasPrefix(arg, "-D") && len(arg) > 2:
			out.Defines = append(out.Defines, arg[2:])
		case strings.HasPrefix(arg, "/D") && len(arg) > 2:
			out.Defines = append(out.Defines, arg[2:])
		case arg == "-U":
			if i+1 < len(args) {
				i++
				out.Undefines = append(out.Undefines, args[i])
			}
		case strings.HasPrefix(arg, "-U") && len(arg) > 2:
			out.Undefines = append(out.Undefines, arg[2:])
		case strings.HasPrefix(arg, "-std="):
			out.Standard = strings.TrimPrefix(arg, "-std=")
		case strings.HasPrefix(arg, "--std="):
			out.Standard = strings.TrimPrefix(arg, "--std=")
		}
	}
}

func splitCompileCommandLine(command string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	var quote rune
	for _, ch := range command {
		switch {
		case inQuote:
			if ch == quote {
				inQuote = false
				continue
			}
			current.WriteRune(ch)
		case ch == '"' || ch == '\'':
			inQuote = true
			quote = ch
		case ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func resolveCompileCommandPath(directory, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(directory, path))
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
