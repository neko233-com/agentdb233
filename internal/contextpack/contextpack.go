package contextpack

import (
	"fmt"
	"strings"

	"github.com/neko233-com/agentdb233/internal/indexer"
	"github.com/neko233-com/agentdb233/internal/model"
)

type Pack struct {
	Project   string                 `json:"project"`
	Query     string                 `json:"query"`
	Budget    int                    `json:"budget"`
	Text      string                 `json:"text"`
	Results   []indexer.Result       `json:"results"`
	Knowledge []model.KnowledgeEntry `json:"knowledge,omitempty"`
}

func Build(project, query string, budget int, results []indexer.Result, knowledge []model.KnowledgeEntry) Pack {
	if budget <= 0 {
		budget = 8000
	}
	var b strings.Builder
	b.WriteString("# agentdb233 context pack\n\n")
	b.WriteString("project: " + project + "\n")
	b.WriteString("query: " + query + "\n\n")
	if len(knowledge) > 0 {
		b.WriteString("## Shared knowledge\n\n")
		for _, k := range knowledge {
			writeBudgeted(&b, budget, fmt.Sprintf("- [%s] %s: %s\n", k.Kind, k.Title, oneLine(k.Body)))
		}
		b.WriteString("\n")
	}
	if len(results) > 0 {
		b.WriteString("## Retrieved code/docs\n\n")
	}
	seen := map[string]bool{}
	var kept []indexer.Result
	for _, r := range results {
		key := fmt.Sprintf("%s:%d:%d", r.Path, r.StartLine, r.EndLine)
		if seen[key] {
			continue
		}
		seen[key] = true
		block := fmt.Sprintf("### %s:%d-%d `%s`\nscore: %d\nsymbol: %s\n\n```%s\n%s\n```\n\n",
			r.Path, r.StartLine, r.EndLine, r.Language, r.Score, r.Symbol, fenceLang(r.Language), trimText(r.Text, 2000))
		if !writeBudgeted(&b, budget, block) {
			break
		}
		kept = append(kept, r)
	}
	return Pack{Project: project, Query: query, Budget: budget, Text: b.String(), Results: kept, Knowledge: knowledge}
}

func writeBudgeted(b *strings.Builder, budget int, text string) bool {
	if b.Len()+len(text) > budget {
		remain := budget - b.Len()
		if remain > 200 {
			b.WriteString(text[:remain])
		}
		return false
	}
	b.WriteString(text)
	return true
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return trimText(s, 500)
}

func trimText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...truncated..."
}

func fenceLang(lang string) string {
	switch lang {
	case "csharp":
		return "csharp"
	case "typescript":
		return "ts"
	case "javascript":
		return "js"
	case "python":
		return "py"
	default:
		return lang
	}
}
