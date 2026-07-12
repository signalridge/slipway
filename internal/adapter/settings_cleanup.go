package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/signalridge/slipway/internal/fsutil"
)

const (
	codexBegin = "# BEGIN SLIPWAY MANAGED CODEX HOOKS"
	codexEnd   = "# END SLIPWAY MANAGED CODEX HOOKS"
)

var codexBeginLine = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(codexBegin) + `\r?$`)
var codexEndLine = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(codexEnd) + `\r?$`)

type settingsChange struct {
	Relative     string
	Data         []byte
	Removed      bool
	sourceSHA256 string
}

func planSettingsCleanup(root string, host Host) (*settingsChange, string) {
	if host.SettingsPath == "" || host.SettingsKind == "preserve" {
		return nil, ""
	}
	path := filepath.Join(root, filepath.FromSlash(host.SettingsPath))
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ""
	}
	if err != nil {
		return nil, fmt.Sprintf("could not inspect %s: %v", host.SettingsPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Sprintf("preserved non-regular settings file %s", host.SettingsPath)
	}
	raw, err := fsutil.ReadFileNoSymlink(path)
	if err != nil {
		return nil, fmt.Sprintf("could not read %s: %v", host.SettingsPath, err)
	}
	switch host.SettingsKind {
	case "json-hooks":
		updated, changed, err := cleanJSONHooks(raw, host.ID, root)
		if err != nil {
			return nil, fmt.Sprintf("preserved malformed settings %s: %v", host.SettingsPath, err)
		}
		if changed {
			return &settingsChange{Relative: host.SettingsPath, Data: updated, Removed: true, sourceSHA256: hashBytes(raw)}, ""
		}
	case "codex-block":
		updated, changed, err := removeManagedBlock(raw)
		if err != nil {
			return nil, fmt.Sprintf("preserved ambiguous settings %s: %v", host.SettingsPath, err)
		}
		if changed {
			return &settingsChange{Relative: host.SettingsPath, Data: updated, Removed: true, sourceSHA256: hashBytes(raw)}, ""
		}
	}
	return nil, ""
}

func cleanJSONHooks(raw []byte, hostID, root string) ([]byte, bool, error) {
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, false, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, false, errors.New("multiple JSON values")
		}
		return nil, false, fmt.Errorf("trailing JSON data: %w", err)
	}
	hooks, ok := document["hooks"].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	targets := []string{"SessionStart"}
	if hostID == "claude" {
		targets = append(targets, "PostToolUse")
	}
	changed := false
	for _, target := range targets {
		value, exists := hooks[target]
		if !exists {
			continue
		}
		cleaned, removed, empty := cleanHookNode(value, root, hostID, target)
		if !removed {
			continue
		}
		changed = true
		if empty {
			delete(hooks, target)
		} else {
			hooks[target] = cleaned
		}
	}
	if !changed {
		return nil, false, nil
	}
	if len(hooks) == 0 {
		delete(document, "hooks")
	}
	updated, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(updated, '\n'), true, nil
}

func cleanHookNode(value any, root, hostID, eventName string) (any, bool, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if command, ok := typed["command"].(string); ok && isRetiredHook(command, root, hostID, eventName) {
			return nil, true, true
		}
		copyMap := make(map[string]any, len(typed))
		for key, child := range typed {
			copyMap[key] = child
		}
		removedAny := false
		for _, key := range []string{"hooks", "commands"} {
			child, exists := typed[key]
			if !exists {
				continue
			}
			cleaned, removed, empty := cleanHookNode(child, root, hostID, eventName)
			removedAny = removedAny || removed
			if removed && empty {
				delete(copyMap, key)
			} else {
				copyMap[key] = cleaned
			}
		}
		if removedAny && (len(copyMap) == 0 || (len(copyMap) == 1 && copyMap["matcher"] != nil)) {
			return nil, true, true
		}
		return copyMap, removedAny, false
	case []any:
		result := make([]any, 0, len(typed))
		removedAny := false
		for _, child := range typed {
			cleaned, removed, empty := cleanHookNode(child, root, hostID, eventName)
			removedAny = removedAny || removed
			if removed && empty {
				continue
			}
			result = append(result, cleaned)
		}
		return result, removedAny, len(result) == 0
	default:
		return value, false, false
	}
}

func isRetiredHook(command, root, hostID, eventName string) bool {
	spec, ok := retiredHookFor(hostID, eventName)
	if !ok {
		return false
	}
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return false
	}
	fields = trimRetiredNoopSuffix(fields)
	if len(fields) == 0 {
		return false
	}
	first := cleanCommandToken(fields[0])
	firstBase := commandBase(first)
	if firstBase == "bash" || firstBase == "sh" {
		script, ok := firstShellScriptArgument(fields[1:])
		return ok && matchesRetiredLauncher(script, root, spec.launcherBase)
	}
	if matchesRetiredLauncher(first, root, spec.launcherBase) {
		return len(fields) == 1
	}
	if strings.EqualFold(first, "slipway") || strings.EqualFold(first, "slipway.exe") {
		if len(fields) < 3 || cleanCommandToken(fields[1]) != "hook" || strings.ToLower(cleanCommandToken(fields[2])) != spec.commandEvent {
			return false
		}
		return retiredHookInvocationSuffix(spec.commandEvent, hostID, fields[3:])
	}
	if firstBase == "go" && len(fields) >= 7 {
		checkout := filepath.Clean(cleanCommandToken(fields[2]))
		canonicalRoot := filepath.Clean(root)
		return filepath.IsAbs(checkout) && filepath.IsAbs(canonicalRoot) && checkout == canonicalRoot &&
			cleanCommandToken(fields[1]) == "-C" &&
			cleanCommandToken(fields[3]) == "run" &&
			cleanCommandToken(fields[4]) == "." &&
			cleanCommandToken(fields[5]) == "hook" &&
			strings.ToLower(cleanCommandToken(fields[6])) == spec.commandEvent &&
			retiredHookInvocationSuffix(spec.commandEvent, hostID, fields[7:])
	}
	return false
}

type retiredHookSpec struct {
	commandEvent string
	launcherBase string
}

func retiredHookFor(hostID, eventName string) (retiredHookSpec, bool) {
	switch hostID + ":" + eventName {
	case "claude:SessionStart":
		return retiredHookSpec{commandEvent: "session-start", launcherBase: ".claude/hooks/slipway-session-start"}, true
	case "claude:PostToolUse":
		return retiredHookSpec{commandEvent: "context-pressure", launcherBase: ".claude/hooks/slipway-context-pressure-post-tool-use"}, true
	case "qwen:SessionStart":
		return retiredHookSpec{commandEvent: "session-start", launcherBase: ".qwen/hooks/slipway-session-start"}, true
	default:
		return retiredHookSpec{}, false
	}
}

func matchesRetiredLauncher(token, root, basePath string) bool {
	token = normalizeHookCommandPath(cleanCommandToken(token))
	basePath = normalizeHookCommandPath(basePath)
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rootPath = strings.TrimSuffix(normalizeHookCommandPath(rootPath), "/")
	projectVariables := []string{}
	if strings.HasPrefix(basePath, ".claude/") {
		projectVariables = []string{"$CLAUDE_PROJECT_DIR/", "${CLAUDE_PROJECT_DIR}/"}
	} else if strings.HasPrefix(basePath, ".qwen/") {
		projectVariables = []string{"$QWEN_PROJECT_DIR/", "${QWEN_PROJECT_DIR}/"}
	}
	for _, suffix := range []string{"", ".sh", ".ps1", ".cmd"} {
		variant := basePath + suffix
		if token == variant {
			return true
		}
		for _, prefix := range projectVariables {
			if token == prefix+variant {
				return true
			}
		}
		absolute := rootPath + "/" + variant
		if token == absolute || (runtime.GOOS == "windows" && strings.EqualFold(token, absolute)) {
			return true
		}
	}
	return false
}

func normalizeHookCommandPath(value string) string {
	return strings.ReplaceAll(filepath.ToSlash(strings.TrimSpace(value)), `\`, "/")
}

func trimRetiredNoopSuffix(fields []string) []string {
	if len(fields) < 3 {
		return fields
	}
	last := len(fields)
	if fields[last-3] == "||" && commandBase(fields[last-2]) == "exit" && cleanCommandToken(fields[last-1]) == "0" {
		return fields[:last-3]
	}
	return fields
}

func retiredHookInvocationSuffix(event, hostID string, fields []string) bool {
	if !retiredHookEvent(event) {
		return false
	}
	if len(fields) == 0 {
		return true
	}
	return event == "session-start" && len(fields) == 2 &&
		cleanCommandToken(fields[0]) == "--tool" &&
		strings.EqualFold(cleanCommandToken(fields[1]), hostID)
}

func cleanCommandToken(token string) string {
	return strings.Trim(strings.TrimSpace(token), `"'`)
}

func commandBase(token string) string {
	token = strings.ReplaceAll(cleanCommandToken(token), `\`, "/")
	if cut := strings.LastIndexByte(token, '/'); cut >= 0 {
		token = token[cut+1:]
	}
	return strings.ToLower(token)
}

func retiredHookEvent(token string) bool {
	token = strings.ToLower(cleanCommandToken(token))
	return token == "session-start" || token == "context-pressure"
}

func firstShellScriptArgument(fields []string) (string, bool) {
	for index, field := range fields {
		argument := cleanCommandToken(field)
		if argument == "" || argument == "--" {
			continue
		}
		if strings.HasPrefix(argument, "-") {
			if strings.Contains(argument, "c") {
				return "", false
			}
			continue
		}
		return argument, index == len(fields)-1
	}
	return "", false
}

func removeManagedBlock(raw []byte) ([]byte, bool, error) {
	begins := codexBeginLine.FindAllIndex(raw, -1)
	ends := codexEndLine.FindAllIndex(raw, -1)
	if len(begins) == 0 && len(ends) == 0 {
		return nil, false, nil
	}
	if len(begins) != 1 || len(ends) != 1 {
		return nil, false, fmt.Errorf("managed block markers are unbalanced")
	}
	start := begins[0][0]
	finish := ends[0][1]
	if finish < start {
		return nil, false, fmt.Errorf("managed block markers are reversed")
	}
	if finish < len(raw) && raw[finish] == '\n' {
		finish++
	}
	updated := make([]byte, 0, len(raw)-(finish-start))
	updated = append(updated, raw[:start]...)
	updated = append(updated, raw[finish:]...)
	return updated, true, nil
}
