package snapshots

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

var normalizedToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:+\[\]-]{0,127}$`)

type normalizer struct{ redactor secrets.Redactor }

type normalizedOrigin struct {
	SourceID       string
	FieldPath      string
	Scope          string
	PrecedenceRank int
}

type normalizedAttribute struct {
	KeyDigest, KindDigest, SecretCategory, ValueDigest string
	Present                                            bool
}

type entity struct {
	kind, id string
	clients  []string
	records  map[string]struct{}
}

// Compare projects each report into an explicit field allowlist. Non-secret
// semantic values enter only as one-way digests; free text, timestamps, paths,
// and secret payloads never appear in public drift output.
// Reports loaded through Store already passed report schema-major validation.
func Compare(current, baseline model.ScanResult) Diff {
	n := normalizer{redactor: secrets.NewRedactor()}
	after, before := n.entities(current), n.entities(baseline)
	diff := Diff{SchemaVersion: SchemaVersion, Changes: []Change{}}
	for key, value := range after {
		previous, exists := before[key]
		if !exists {
			diff.Changes = append(diff.Changes, change(value, ChangeAdded))
		} else if fingerprint(value) != fingerprint(previous) {
			diff.Changes = append(diff.Changes, change(value, ChangeChanged))
		}
	}
	for key, value := range before {
		if _, exists := after[key]; !exists {
			diff.Changes = append(diff.Changes, change(value, ChangeRemoved))
		}
	}
	sort.Slice(diff.Changes, func(i, j int) bool {
		a, b := diff.Changes[i], diff.Changes[j]
		if a.EntityType != b.EntityType {
			return a.EntityType < b.EntityType
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Kind < b.Kind
	})
	return diff
}

func change(value *entity, kind string) Change {
	return Change{Kind: kind, EntityType: value.kind, ID: value.id,
		Summary: strings.ToLower(value.kind) + " " + value.id + " " + strings.ToLower(kind),
		Clients: value.clients}
}

func (n normalizer) entities(input model.ScanResult) map[string]*entity {
	result := map[string]*entity{}
	add := func(kind, id string, clients []string, fields any) {
		if id == "" {
			id = "unknown"
		}
		key := kind + "\x00" + id
		value := result[key]
		if value == nil {
			value = &entity{kind: kind, id: id, records: map[string]struct{}{}}
			result[key] = value
		}
		value.clients = n.tokens(append(value.clients, clients...))
		value.records[encode(fields)] = struct{}{}
	}
	for _, client := range input.Analysis.Clients {
		clients := n.tokens([]string{client.ID})
		add(EntityClient, n.token(client.ID), clients, struct {
			Installed    bool
			Version      string
			Capabilities model.AdapterCapabilities
		}{client.Detection.Installed, n.token(client.Detection.Version), client.Capabilities})
		compatibility := client.Compatibility
		add(EntityCompatibility, n.token(client.ID), clients, struct {
			Tier, State, InstalledVersion, RulesetVersion string
			VerifiedVersions, SupportedFeatures           []string
		}{n.token(string(compatibility.Tier)), n.token(string(compatibility.State)), n.token(compatibility.InstalledVersion), n.token(compatibility.RulesetVersion), n.tokens(compatibility.VerifiedVersions), n.tokens(compatibility.SupportedFeatures)})
	}
	for _, source := range input.Analysis.Sources {
		add(EntitySource, n.token(source.ID), []string{source.Client}, struct {
			Client, Scope, Format, Kind string
			Exists, Readable, Symlink   bool
		}{n.token(source.Client), n.token(string(source.Scope)), n.token(string(source.Format)), n.token(source.Kind), source.Exists, source.Readable, source.Symlink})
	}
	for _, node := range input.Analysis.Graph.Nodes {
		add(EntityNode, n.token(node.ID), []string{node.Client}, struct {
			Type, Client, Confidence               string
			DisplayNameDigest, LoadConditionDigest string
			Attributes                             []normalizedAttribute
			Origins                                []normalizedOrigin
		}{n.token(string(node.Type)), n.token(node.Client), n.token(string(node.AdapterConfidence)), digest(node.DisplayName), digest(node.LoadCondition), n.attributes(node.Attributes), n.origins(node.Origins)})
	}
	for _, finding := range input.Analysis.Findings {
		origins := n.origins(finding.Origins)
		locations := make([]string, 0, len(origins))
		for _, origin := range origins {
			locations = append(locations, encode(struct{ SourceID, FieldPath string }{origin.SourceID, origin.FieldPath}))
		}
		locations = sortedUnique(locations)
		ruleID := n.token(finding.RuleID)
		if ruleID == "" {
			ruleID = "unknown"
		}
		identity := encode(struct {
			References, Locations []string
		}{n.tokens(finding.GraphReferences), locations})
		id := ruleID + ":" + digest(identity)[:16]
		add(EntityFinding, id, finding.AffectedClients, struct {
			RuleID, Severity, Evidence string
			Clients, References        []string
			Origins                    []normalizedOrigin
		}{ruleID, n.token(string(finding.Severity)), n.token(string(finding.Evidence)), n.tokens(finding.AffectedClients), n.tokens(finding.GraphReferences), origins})
	}
	return result
}

func (n normalizer) attributes(input map[string]model.SafeValue) []normalizedAttribute {
	values := make([]normalizedAttribute, 0, len(input))
	for key, value := range input {
		redacted := n.redactor.RedactField(key, value.Display)
		secretCategory := value.SecretCategory
		if secretCategory == "" {
			secretCategory = redacted.SecretCategory
		}
		normalized := normalizedAttribute{
			KeyDigest: digest(key), KindDigest: digest(value.Kind),
			SecretCategory: n.token(secretCategory), Present: value.Present,
		}
		if secretCategory == "" && redacted.Display == value.Display {
			normalized.ValueDigest = digest(value.Display)
		}
		values = append(values, normalized)
	}
	sort.Slice(values, func(i, j int) bool { return encode(values[i]) < encode(values[j]) })
	return values
}

func (n normalizer) token(input string) string {
	value := strings.TrimSpace(input)
	if !normalizedToken.MatchString(value) || n.redactor.RedactField("identifier", value).Display != value {
		return ""
	}
	return value
}

func (n normalizer) tokens(input []string) []string {
	values := make([]string, 0, len(input))
	for _, value := range input {
		if safe := n.token(value); safe != "" {
			values = append(values, safe)
		}
	}
	return sortedUnique(values)
}

func (n normalizer) origins(input []model.Origin) []normalizedOrigin {
	values := map[string]normalizedOrigin{}
	for _, origin := range input {
		value := normalizedOrigin{SourceID: n.token(origin.SourceID), FieldPath: n.token(origin.FieldPath), Scope: n.token(string(origin.Scope)), PrecedenceRank: origin.PrecedenceRank}
		values[encode(value)] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	result := make([]normalizedOrigin, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func fingerprint(value *entity) string {
	records := make([]string, 0, len(value.records))
	for record := range value.records {
		records = append(records, record)
	}
	slices.Sort(records)
	return digest(encode(records))
}

func sortedUnique(input []string) []string {
	slices.Sort(input)
	return slices.Compact(input)
}

// encode only receives the fixed primitive projections declared above.
func encode(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func digest(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
