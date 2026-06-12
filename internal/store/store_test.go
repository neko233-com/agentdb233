package store

import (
	"testing"

	"github.com/neko233-com/agentdb233/internal/model"
)

func TestKnowledgeSkillMCP(t *testing.T) {
	st := New(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddKnowledge(model.KnowledgeEntry{
		Project: "demo",
		Kind:    "norm",
		Title:   "Branch rule",
		Body:    "feature branches use feat/name",
		Tags:    []string{"git"},
	}); err != nil {
		t.Fatal(err)
	}
	found, err := st.ListKnowledge("demo", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("knowledge=%d want 1", len(found))
	}
	if _, err := st.UpsertSkill(model.Skill{Name: "game-lore", Path: "SKILL.md"}); err != nil {
		t.Fatal(err)
	}
	skills, err := st.ListSkills()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "game-lore" {
		t.Fatalf("skills=%v", skills)
	}
	if _, err := st.UpsertMCP(model.MCPEndpoint{Name: "unity", Command: "mcp-unity"}); err != nil {
		t.Fatal(err)
	}
	mcps, err := st.ListMCP()
	if err != nil {
		t.Fatal(err)
	}
	if len(mcps) != 1 || mcps[0].Name != "unity" {
		t.Fatalf("mcps=%v", mcps)
	}
}
