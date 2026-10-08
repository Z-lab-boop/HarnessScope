package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Z-lab-boop/harnessscope/internal/model"
	"github.com/Z-lab-boop/harnessscope/internal/secrets"
)

func WriteJSON(output io.Writer, result model.ScanResult) error {
	data, err := safeCanonicalJSON(result)
	if err != nil {
		return err
	}
	_, err = output.Write(data)
	return err
}

func ReadJSON(input io.Reader) (model.ScanResult, error) {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	var result model.ScanResult
	if err := decoder.Decode(&result); err != nil {
		return model.ScanResult{}, fmt.Errorf("decode report JSON: %w", err)
	}
	major := strings.SplitN(result.SchemaVersion, ".", 2)[0]
	if major != "1" {
		return model.ScanResult{}, fmt.Errorf("unsupported major schema version %q", result.SchemaVersion)
	}
	return model.Canonicalize(result), nil
}

func safeCanonicalJSON(result model.ScanResult) ([]byte, error) {
	data, err := model.MarshalCanonical(result)
	if err != nil {
		return nil, err
	}
	scrubbed := secrets.NewRedactor().ScrubText(string(data))
	return []byte(scrubbed), nil
}
