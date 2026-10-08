package claude

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Z-lab-boop/harnessscope/internal/discovery"
	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

const (
	clientID        = "claude"
	verifiedVersion = "2.1.259"
	rulesetVersion  = "claude-2026-10-08"
)

type Option func(*Adapter)

func WithManagedSettingsPath(path string) Option {
	return func(adapter *Adapter) { adapter.managedSettingsPath = path }
}

type Adapter struct {
	env                 discovery.Environment
	redactor            secrets.Redactor
	managedSettingsPath string
	mu                  sync.RWMutex
	detected            model.DetectionResult
}

func New(env discovery.Environment, redactor secrets.Redactor, options ...Option) *Adapter {
	managed := "/etc/claude-code/managed-settings.json"
	if env.GOOS == "darwin" {
		managed = "/Library/Application Support/ClaudeCode/managed-settings.json"
	}
	adapter := &Adapter{env: env, redactor: redactor, managedSettingsPath: managed}
	for _, option := range options {
		option(adapter)
	}
	return adapter
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
			fields := strings.Fields(string(output))
			if len(fields) > 0 {
				result.Version = fields[0]
			}
		}
	}
	a.mu.Lock()
	a.detected = result
	a.mu.Unlock()
	return result
}

func (a *Adapter) Capabilities() model.AdapterCapabilities {
	return model.AdapterCapabilities{Precedence: true, Instructions: true, Skills: true, Hooks: true, MCP: true}
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
		Tier: model.TierVerified, State: state, InstalledVersion: detection.Version,
		VerifiedVersions: []string{verifiedVersion}, RulesetVersion: rulesetVersion,
		LastVerificationAt: "2026-10-08",
		EvidenceReferences: []string{
			"https://code.claude.com/docs/en/memory",
			"https://code.claude.com/docs/en/settings",
			"https://code.claude.com/docs/en/mcp",
		},
		SupportedFeatures: []string{"settings", "instructions", "imports", "hooks", "mcp", "precedence"},
	}
}

func (a *Adapter) managedSource() model.ConfigSource {
	return a.source(filepath.Clean(a.managedSettingsPath), model.ScopeManaged, model.FormatJSON, "settings", "Claude managed settings")
}
