package report

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io"
	"math"

	"github.com/Z-lab-boop/harnessscope/internal/model"
)

//go:embed assets/report.css
var reportCSS string

//go:embed assets/report.js
var reportJS string

type graphPoint struct {
	X, Y  int
	Label string
}

type htmlView struct {
	CSS      template.CSS
	JS       template.JS
	Data     string
	Clients  []model.ClientResult
	Findings []model.Finding
	Points   []graphPoint
}

var htmlTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>HarnessScope report</title><style>{{.CSS}}</style></head>
<body><header><p class="tier">HARNESSCOPE · OFFLINE REPORT</p><h1>Effective configuration provenance</h1><p class="muted">See what your coding agent actually loads.</p></header>
<main><section><h2>Clients</h2><div class="cards">{{range .Clients}}<article class="card"><strong>{{.ID}}</strong><p>{{.Compatibility.Tier}} · {{.Compatibility.State}}</p><p class="muted">{{.Detection.Version}}</p></article>{{else}}<p>No clients detected.</p>{{end}}</div></section>
<section><h2>Configuration graph</h2><svg viewBox="0 0 900 220" role="img" aria-label="Configuration graph node overview">{{range .Points}}<circle cx="{{.X}}" cy="{{.Y}}" r="8"></circle><text x="{{.X}}" y="{{.Y}}" dx="12" dy="4">{{.Label}}</text>{{else}}<text x="20" y="40">No graph nodes.</text>{{end}}</svg></section>
<section id="findings"><h2>Findings</h2><label for="severity-filter">Severity </label><select id="severity-filter"><option>ALL</option><option>HIGH</option><option>MEDIUM</option><option>LOW</option><option>INFO</option></select>
<table><thead><tr><th>Severity</th><th>Rule</th><th>Finding</th><th>Evidence</th><th>Action</th></tr></thead><tbody>{{range .Findings}}<tr data-severity="{{.Severity}}"><td class="severity {{.Severity}}">{{.Severity}}</td><td>{{.RuleID}}</td><td><strong>{{.Summary}}</strong><br>{{.Reason}}<br><span class="muted">{{.Impact}}</span></td><td>{{.Evidence}}</td><td>{{.Remediation}}</td></tr>{{else}}<tr><td colspan="5">No findings.</td></tr>{{end}}</tbody></table></section>
<section><button id="toggle-data" type="button">Toggle embedded JSON</button><pre id="raw-data" hidden>{{.Data}}</pre></section></main>
<script id="report-data" type="application/json" data-encoding="base64">{{.Data}}</script><script>{{.JS}}</script></body></html>`))

func WriteHTML(output io.Writer, result model.ScanResult) error {
	data, err := safeCanonicalJSON(result)
	if err != nil {
		return err
	}
	var safe model.ScanResult
	if err := json.Unmarshal(data, &safe); err != nil {
		return err
	}
	view := htmlView{
		CSS: template.CSS(reportCSS), JS: template.JS(reportJS),
		Data:    base64.StdEncoding.EncodeToString(data),
		Clients: safe.Analysis.Clients, Findings: safe.Analysis.Findings,
		Points: graphPoints(safe.Analysis.Graph.Nodes),
	}
	return htmlTemplate.Execute(output, view)
}

func graphPoints(nodes []model.ConfigNode) []graphPoint {
	limit := len(nodes)
	if limit > 20 {
		limit = 20
	}
	points := make([]graphPoint, 0, limit)
	for index := 0; index < limit; index++ {
		angle := float64(index) * 2 * math.Pi / math.Max(1, float64(limit))
		points = append(points, graphPoint{
			X: 450 + int(300*math.Cos(angle)), Y: 110 + int(80*math.Sin(angle)), Label: nodes[index].DisplayName,
		})
	}
	return points
}
