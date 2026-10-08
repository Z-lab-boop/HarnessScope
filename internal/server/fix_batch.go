package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/Z-lab-boop/harnessscope/internal/fixes"
	"github.com/Z-lab-boop/harnessscope/internal/model"
)

type fixFileState struct {
	data []byte
	mode os.FileMode
}

type fixStep struct {
	plan          model.FixPlan
	before, after map[string]fixFileState
}

func fixHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// prepareFixSteps preserves the selected edits themselves, rather than matching
// them to new plans. Simulating disjoint edits rebases their exact byte ranges
// and pins every later hash to the bytes this batch is expected to produce.
// Overlapping ranges or unknown operations fail before the first write.
func prepareFixSteps(selected []model.FixPlan) ([]fixStep, error) {
	plans := append([]model.FixPlan(nil), selected...)
	files := map[string]fixFileState{}
	for i := range plans {
		plans[i].Edits = append([]model.Edit(nil), plans[i].Edits...)
		for _, edit := range plans[i].Edits {
			file, exists := files[edit.TargetPath]
			if !exists {
				data, err := os.ReadFile(edit.TargetPath)
				if err != nil {
					return nil, fixes.ErrTargetChanged
				}
				info, err := os.Stat(edit.TargetPath)
				if err != nil || !info.Mode().IsRegular() {
					return nil, fixes.ErrTargetChanged
				}
				file = fixFileState{data, info.Mode().Perm()}
				files[edit.TargetPath] = file
			}
			if fixHash(file.data) != edit.ExpectedHash {
				return nil, fixes.ErrTargetChanged
			}
		}
	}
	steps := make([]fixStep, 0, len(plans))
	for i, plan := range plans {
		step := fixStep{plan: plan, before: map[string]fixFileState{}, after: map[string]fixFileState{}}
		for j := range plan.Edits {
			edit := &plan.Edits[j]
			file := files[edit.TargetPath]
			if _, exists := step.before[edit.TargetPath]; !exists {
				step.before[edit.TargetPath] = file
			}
			edit.ExpectedHash = fixHash(step.before[edit.TargetPath].data)
			switch edit.Operation {
			case "chmod_executable":
				edit.OriginalMode = uint32(file.mode)
				file.mode = os.FileMode(edit.ResultMode)
			case "remove_byte_range", "normalize_path":
				start, end := edit.StartOffset, edit.EndOffset
				if start < 0 || end <= start || end > len(file.data) {
					return nil, fixes.ErrTargetChanged
				}
				replacement := []byte(nil)
				if edit.Operation == "normalize_path" {
					replacement = []byte(edit.Replacement)
				}
				delta := len(replacement) - (end - start)
				// Include later edits of this transaction, preserving their order
				// and rejecting any edit that would consume a selected range twice.
				for next := i; next < len(plans); next++ {
					for index := range plans[next].Edits {
						if next == i && index <= j {
							continue
						}
						pending := &plans[next].Edits[index]
						if pending.TargetPath != edit.TargetPath || pending.Operation == "chmod_executable" {
							continue
						}
						if pending.EndOffset <= start {
							continue
						}
						if pending.StartOffset < end {
							return nil, fixes.ErrTargetChanged
						}
						pending.StartOffset += delta
						pending.EndOffset += delta
					}
				}
				updated := make([]byte, 0, len(file.data)+delta)
				updated = append(updated, file.data[:start]...)
				updated = append(updated, replacement...)
				file.data = append(updated, file.data[end:]...)
			default:
				return nil, fixes.ErrTargetChanged
			}
			files[edit.TargetPath] = file
			step.after[edit.TargetPath] = file
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func verifyFixFiles(expected map[string]fixFileState) error {
	for path, file := range expected {
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, file.data) {
			return fixes.ErrTargetChanged
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != file.mode {
			return fixes.ErrTargetChanged
		}
	}
	return nil
}
