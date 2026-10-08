package model

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const ReportSchemaVersion = "1.0.0"

type ClientTier string

const (
	TierVerified ClientTier = "VERIFIED"
	TierPreview  ClientTier = "PREVIEW"
)

type Scope string

const (
	ScopeSystem  Scope = "SYSTEM"
	ScopeManaged Scope = "MANAGED"
	ScopeUser    Scope = "USER"
	ScopeProfile Scope = "PROFILE"
	ScopeProject Scope = "PROJECT"
	ScopeNested  Scope = "NESTED"
	ScopeLocal   Scope = "LOCAL"
	ScopeBuiltin Scope = "BUILTIN"
)

type SourceFormat string

const (
	FormatJSON     SourceFormat = "JSON"
	FormatJSONC    SourceFormat = "JSONC"
	FormatTOML     SourceFormat = "TOML"
	FormatYAML     SourceFormat = "YAML"
	FormatMarkdown SourceFormat = "MARKDOWN"
	FormatUnknown  SourceFormat = "UNKNOWN"
)

type NodeType string

const (
	NodeClient      NodeType = "CLIENT"
	NodeScope       NodeType = "SCOPE"
	NodeSource      NodeType = "SOURCE"
	NodeInstruction NodeType = "INSTRUCTION"
	NodeRule        NodeType = "RULE"
	NodeSkill       NodeType = "SKILL"
	NodeHook        NodeType = "HOOK"
	NodeMCPServer   NodeType = "MCP_SERVER"
	NodeEnvironment NodeType = "ENVIRONMENT_REFERENCE"
	NodeEffective   NodeType = "EFFECTIVE_OUTPUT"
)

type EdgeType string

const (
	EdgeLoads       EdgeType = "LOADS"
	EdgeImports     EdgeType = "IMPORTS"
	EdgeOverrides   EdgeType = "OVERRIDES"
	EdgeShadows     EdgeType = "SHADOWS"
	EdgeDuplicates  EdgeType = "DUPLICATES"
	EdgeReferences  EdgeType = "REFERENCES"
	EdgeEffectiveAs EdgeType = "EFFECTIVE_AS"
)

type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
	SeverityInfo   Severity = "INFO"
)

type EvidenceStatus string

const (
	EvidenceConfirmed EvidenceStatus = "CONFIRMED"
	EvidenceLikely    EvidenceStatus = "LIKELY"
	EvidenceUnknown   EvidenceStatus = "UNKNOWN"
)

type RiskClass string

const (
	RiskSafe   RiskClass = "SAFE"
	RiskReview RiskClass = "REVIEW"
	RiskManual RiskClass = "MANUAL"
)

type CompatibilityState string

const (
	CompatibilityVerified CompatibilityState = "VERIFIED"
	CompatibilityUnknown  CompatibilityState = "COMPATIBILITY_UNKNOWN"
)

type SafeValue struct {
	Kind           string `json:"kind"`
	Display        string `json:"display"`
	SecretCategory string `json:"secret_category,omitempty"`
	Present        bool   `json:"present"`
}

type DetectionResult struct {
	Installed  bool   `json:"installed"`
	Executable string `json:"executable,omitempty"`
	Version    string `json:"version,omitempty"`
	Error      string `json:"error,omitempty"`
}

type AdapterCapabilities struct {
	Precedence   bool `json:"precedence"`
	Instructions bool `json:"instructions"`
	Skills       bool `json:"skills"`
	Hooks        bool `json:"hooks"`
	MCP          bool `json:"mcp"`
}

type CompatibilityMetadata struct {
	Tier               ClientTier         `json:"tier"`
	State              CompatibilityState `json:"state"`
	InstalledVersion   string             `json:"installed_version,omitempty"`
	VerifiedVersions   []string           `json:"verified_versions,omitempty"`
	RulesetVersion     string             `json:"ruleset_version"`
	LastVerificationAt string             `json:"last_verification_at"`
	EvidenceReferences []string           `json:"evidence_references,omitempty"`
	SupportedFeatures  []string           `json:"supported_features,omitempty"`
}

type ConfigSource struct {
	ID              string       `json:"id"`
	LogicalPath     string       `json:"logical_path"`
	CanonicalPath   string       `json:"canonical_path,omitempty"`
	Client          string       `json:"client"`
	Scope           Scope        `json:"scope"`
	Format          SourceFormat `json:"format"`
	Exists          bool         `json:"exists"`
	Readable        bool         `json:"readable"`
	Kind            string       `json:"kind"`
	DiscoveryReason string       `json:"discovery_reason"`
	Symlink         bool         `json:"symlink"`
}

type Origin struct {
	SourceID       string `json:"source_id"`
	LogicalPath    string `json:"logical_path,omitempty"`
	Line           int    `json:"line,omitempty"`
	Column         int    `json:"column,omitempty"`
	FieldPath      string `json:"field_path,omitempty"`
	Scope          Scope  `json:"scope"`
	PrecedenceRank int    `json:"precedence_rank,omitempty"`
	Rule           string `json:"rule"`
}

type ConfigNode struct {
	ID                string               `json:"id"`
	Type              NodeType             `json:"type"`
	Client            string               `json:"client"`
	DisplayName       string               `json:"display_name"`
	Attributes        map[string]SafeValue `json:"attributes,omitempty"`
	Origins           []Origin             `json:"origins,omitempty"`
	LoadCondition     string               `json:"load_condition,omitempty"`
	AdapterConfidence EvidenceStatus       `json:"adapter_confidence"`
}

type Edge struct {
	ID       string         `json:"id"`
	From     string         `json:"from"`
	To       string         `json:"to"`
	Type     EdgeType       `json:"type"`
	Ruleset  string         `json:"ruleset"`
	Evidence EvidenceStatus `json:"evidence"`
	Origin   *Origin        `json:"origin,omitempty"`
}

type Graph struct {
	Nodes []ConfigNode `json:"nodes"`
	Edges []Edge       `json:"edges"`
}

type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	// Behavioral findings inherit the weakest relevant adapter evidence;
	// directly observed source and credential-presence facts may be CONFIRMED.
	Evidence        EvidenceStatus `json:"evidence"`
	Summary         string         `json:"summary"`
	Reason          string         `json:"reason"`
	Impact          string         `json:"impact"`
	AffectedClients []string       `json:"affected_clients,omitempty"`
	Origins         []Origin       `json:"origins,omitempty"`
	GraphReferences []string       `json:"graph_references,omitempty"`
	Remediation     string         `json:"remediation"`
	FixPlanID       string         `json:"fix_plan_id,omitempty"`
}

type ContextEstimate struct {
	Client     string `json:"client"`
	SourceID   string `json:"source_id"`
	Category   string `json:"category"`
	Bytes      int    `json:"bytes"`
	CodePoints int    `json:"code_points"`
	MinTokens  int    `json:"min_tokens"`
	MaxTokens  int    `json:"max_tokens"`
	Method     string `json:"method"`
}

type Edit struct {
	SourceID      string `json:"source_id"`
	TargetPath    string `json:"target_path"`
	ExpectedHash  string `json:"expected_hash"`
	Operation     string `json:"operation"`
	StartOffset   int    `json:"start_offset,omitempty"`
	EndOffset     int    `json:"end_offset,omitempty"`
	Replacement   string `json:"replacement,omitempty"`
	OriginalMode  uint32 `json:"original_mode,omitempty"`
	ResultMode    uint32 `json:"result_mode,omitempty"`
	RedactedPatch string `json:"redacted_patch"`
}

type FixPlan struct {
	ID             string    `json:"id"`
	FindingRuleID  string    `json:"finding_rule_id"`
	Risk           RiskClass `json:"risk"`
	Edits          []Edit    `json:"edits"`
	Preconditions  []string  `json:"preconditions"`
	Postconditions []string  `json:"postconditions"`
	BackupRequired bool      `json:"backup_required"`
	Rollback       []string  `json:"rollback"`
}

type ParsedConfig struct {
	Client      string         `json:"client"`
	Sources     []ConfigSource `json:"sources"`
	Nodes       []ConfigNode   `json:"nodes"`
	Edges       []Edge         `json:"edges"`
	Findings    []Finding      `json:"findings"`
	Limitations []string       `json:"limitations,omitempty"`
}

type EffectiveConfig struct {
	Client      string         `json:"client"`
	Sources     []ConfigSource `json:"sources"`
	Nodes       []ConfigNode   `json:"nodes"`
	Edges       []Edge         `json:"edges"`
	Findings    []Finding      `json:"findings"`
	Limitations []string       `json:"limitations,omitempty"`
}

type ClientResult struct {
	ID            string                `json:"id"`
	Detection     DetectionResult       `json:"detection"`
	Capabilities  AdapterCapabilities   `json:"capabilities"`
	Compatibility CompatibilityMetadata `json:"compatibility"`
	Effective     EffectiveConfig       `json:"effective"`
}

type Analysis struct {
	Clients  []ClientResult    `json:"clients"`
	Sources  []ConfigSource    `json:"sources"`
	Graph    Graph             `json:"graph"`
	Findings []Finding         `json:"findings"`
	Context  []ContextEstimate `json:"context,omitempty"`
	FixPlans []FixPlan         `json:"fix_plans,omitempty"`
}

type RunMetadata struct {
	// Executable search entries are private analyzer inputs and are never serialized.
	GeneratedAt string `json:"generated_at,omitempty"`
	CWD         string `json:"cwd,omitempty"`
	OS          string `json:"os,omitempty"`
}

type ScanResult struct {
	SchemaVersion string       `json:"schema_version"`
	Analysis      Analysis     `json:"analysis"`
	RunMetadata   *RunMetadata `json:"run_metadata,omitempty"`
}

func StableNodeID(client, sourceIdentity, fieldPath string) string {
	return stableID("node", client, sourceIdentity, fieldPath)
}

func StableSourceID(client, sourceIdentity string) string {
	return stableID("source", client, sourceIdentity)
}

func StableEdgeID(from string, edgeType EdgeType, to string) string {
	return stableID("edge", from, string(edgeType), to)
}

func stableID(namespace string, parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return namespace + "_" + hex.EncodeToString(h.Sum(nil))[:16]
}
