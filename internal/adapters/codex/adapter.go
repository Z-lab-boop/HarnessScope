package codex

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

const (
	clientID        = "codex"
	verifiedVersion = "0.162.0-alpha.2"
	rulesetVersion  = "codex-2026-10-08"
)

type Adapter struct {
	env      discovery.Environment
	redactor secrets.Redactor
	mu       sync.RWMutex
	detected model.DetectionResult
}

func New(env discovery.Environment, redactor secrets.Redactor) *Adapter {
	return &Adapter{env: env, redactor: redactor}
}

func (a *Adapter) ID() string { return clientID }

func (a *Adapter) Detect(ctx context.Context) model.DetectionResult {
	result := a.env.FindExecutable(clientID)
	if result.Installed {
		versionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		output, err := exec.CommandContext(versionCtx, result.Executable, "--version").CombinedOutput()
		if err != nil {
			result.Error = a.redactor.ScrubText(err.Error())
		} else {
			result.Version = parseVersion(string(output))
		}
	}
	a.mu.Lock()
	a.detected = result
	a.mu.Unlock()
	return result
}

func (a *Adapter) Capabilities() model.AdapterCapabilities {
	return model.AdapterCapabilities{
		Precedence: true, Instructions: true, Skills: true, Hooks: true, MCP: true,
	}
}

func (a *Adapter) Compatibility() model.CompatibilityMetadata {
	a.mu.RLock()
	detection := a.detected
	a.mu.RUnlock()
	state := model.CompatibilityUnknown
	if detection.Version == verifiedVersion {
		state = model.CompatibilityVerified
	}
	return model.CompatibilityMetadata{
		Tier:               model.TierVerified,
		State:              state,
		InstalledVersion:   detection.Version,
		VerifiedVersions:   []string{verifiedVersion},
		RulesetVersion:     rulesetVersion,
		LastVerificationAt: "2026-10-08",
		EvidenceReferences: []string{
			"https://learn.chatgpt.com/docs/config-file/config-basic",
			"https://learn.chatgpt.com/docs/agent-configuration/agents-md",
		},
		SupportedFeatures: []string{"config", "instructions", "skills", "hooks", "mcp", "precedence"},
	}
}

func parseVersion(output string) string {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}
