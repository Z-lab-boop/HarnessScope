package fixes

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

func Plan(result model.ScanResult, selected []string) ([]model.FixPlan, error) {
	var plans []model.FixPlan
	for _, source := range result.Analysis.Sources {
		plan, ok, err := duplicatePlan(source)
		if err != nil {
			return nil, err
		}
		if ok {
			plans = append(plans, plan)
		}
	}
	for _, node := range result.Analysis.Graph.Nodes {
		if node.Type == model.NodeHook {
			plan, ok, err := hookModePlan(node)
			if err != nil {
				return nil, err
			}
			if ok {
				plans = append(plans, plan)
			}
		}
		plans = append(plans, normalizedPathPlans(node, result.Analysis.Sources)...)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].ID < plans[j].ID })
	if len(selected) == 0 {
		return plans, nil
	}
	wanted := make(map[string]struct{}, len(selected))
	for _, id := range selected {
		wanted[id] = struct{}{}
	}
	filtered := make([]model.FixPlan, 0, len(selected))
	for _, plan := range plans {
		if _, ok := wanted[plan.ID]; ok {
			filtered = append(filtered, plan)
			delete(wanted, plan.ID)
		}
	}
	if len(wanted) > 0 {
		return nil, fmt.Errorf("selected fix ID was not found")
	}
	return filtered, nil
}

func duplicatePlan(source model.ConfigSource) (model.FixPlan, bool, error) {
	if !source.Exists || !source.Readable || source.CanonicalPath == "" {
		return model.FixPlan{}, false, nil
	}
	data, err := os.ReadFile(source.CanonicalPath)
	if err != nil {
		return model.FixPlan{}, false, nil
	}
	seen := make(map[string]struct{})
	var edits []model.Edit
	offset := 0
	for _, line := range bytes.SplitAfter(data, []byte("\n")) {
		text := string(line)
		key := strings.TrimSpace(text)
		if key != "" {
			if _, exists := seen[key]; exists {
				edits = append(edits, model.Edit{
					SourceID: source.ID, TargetPath: source.CanonicalPath, ExpectedHash: hashBytes(data),
					Operation: "remove_byte_range", StartOffset: offset, EndOffset: offset + len(line),
					RedactedPatch: "remove duplicate line: " + secrets.NewRedactor().ScrubText(key),
				})
			} else {
				seen[key] = struct{}{}
			}
		}
		offset += len(line)
	}
	if len(edits) == 0 {
		return model.FixPlan{}, false, nil
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].StartOffset > edits[j].StartOffset })
	plan := safePlan("DUP", source.CanonicalPath, edits[0], "Duplicate lines are still present", "Duplicate lines are removed")
	plan.Edits = edits
	return plan, true, nil
}

func hookModePlan(node model.ConfigNode) (model.FixPlan, bool, error) {
	command, ok := node.Attributes["command"]
	if !ok || !command.Present || !filepath.IsAbs(command.Display) {
		return model.FixPlan{}, false, nil
	}
	data, err := os.ReadFile(command.Display)
	if err != nil || !bytes.HasPrefix(data, []byte("#!")) {
		return model.FixPlan{}, false, nil
	}
	info, err := os.Stat(command.Display)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 != 0 {
		return model.FixPlan{}, false, nil
	}
	edit := model.Edit{
		TargetPath: command.Display, ExpectedHash: hashBytes(data), Operation: "chmod_executable",
		OriginalMode: uint32(info.Mode().Perm()), ResultMode: uint32(info.Mode().Perm() | 0o111),
		RedactedPatch: "add executable bits to an existing shebang hook",
	}
	return safePlan("HOOK", command.Display, edit, "Hook is not executable", "Hook has executable permissions"), true, nil
}

func normalizedPathPlans(node model.ConfigNode, sources []model.ConfigSource) []model.FixPlan {
	var plans []model.FixPlan
	for key, value := range node.Attributes {
		if key != "path" && key != "cwd" && key != "command" || !value.Present || !filepath.IsAbs(value.Display) {
			continue
		}
		cleaned := filepath.Clean(value.Display)
		canonical, err := filepath.EvalSymlinks(value.Display)
		if err != nil {
			continue
		}
		if canonical == value.Display && cleaned == value.Display {
			continue
		}
		oldInfo, oldErr := os.Stat(value.Display)
		newInfo, newErr := os.Stat(canonical)
		if oldErr != nil || newErr != nil || !os.SameFile(oldInfo, newInfo) {
			continue
		}
		for _, origin := range node.Origins {
			source := sourceByID(sources, origin.SourceID)
			if source == nil || source.CanonicalPath == "" {
				continue
			}
			data, err := os.ReadFile(source.CanonicalPath)
			if err != nil {
				continue
			}
			start := bytes.Index(data, []byte(value.Display))
			if start < 0 {
				continue
			}
			edit := model.Edit{
				SourceID: source.ID, TargetPath: source.CanonicalPath, ExpectedHash: hashBytes(data),
				Operation: "normalize_path", StartOffset: start, EndOffset: start + len(value.Display),
				Replacement: canonical, RedactedPatch: "normalize path to the same canonical object",
			}
			plans = append(plans, safePlan("PATH", source.CanonicalPath, edit, "Path is not canonical", "Path resolves to the same canonical object"))
		}
	}
	return plans
}

func safePlan(prefix, identity string, edit model.Edit, precondition, postcondition string) model.FixPlan {
	idHash := sha256.Sum256([]byte(prefix + "\x00" + identity + "\x00" + edit.Operation + fmt.Sprint(edit.StartOffset)))
	return model.FixPlan{
		ID:   "FIX-" + prefix + "-" + strings.ToUpper(hex.EncodeToString(idHash[:])[:8]),
		Risk: model.RiskSafe, Edits: []model.Edit{edit},
		Preconditions: []string{precondition}, Postconditions: []string{postcondition},
		BackupRequired: true, Rollback: []string{"restore the backed-up file and original mode"},
	}
}

func sourceByID(sources []model.ConfigSource, id string) *model.ConfigSource {
	for index := range sources {
		if sources[index].ID == id {
			return &sources[index]
		}
	}
	return nil
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
