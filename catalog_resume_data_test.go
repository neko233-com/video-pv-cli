package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogDataRevalidationNeverAllowsRendererChanges(t *testing.T) {
	root := t.TempDir()
	prior := CatalogEvidence{Schema: "test/1", CaseCount: 1, Mode: "deep", CatalogSHA256: "old-data", SourceSHA256: map[string]string{"renderer": "same"}}
	raw, _ := json.Marshal(prior)
	if err := os.WriteFile(filepath.Join(root, "catalog-report.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	expected := prior
	expected.CatalogSHA256 = "new-data"
	cases := []AtlasCase{{ID: "not-yet-rendered"}}
	if _, err := resumeCatalogEvidence(root, expected, cases, newCatalogPixelGate(), io.Discard); err == nil {
		t.Fatal("implicit data change accepted")
	}
	got, err := resumeCatalogEvidence(root, expected, cases, newCatalogPixelGate(), io.Discard, true)
	if err != nil || got.CatalogSHA256 != "new-data" {
		t.Fatal(got, err)
	}
	expected.SourceSHA256 = map[string]string{"renderer": "changed"}
	if _, err := resumeCatalogEvidence(root, expected, cases, newCatalogPixelGate(), io.Discard, true); err == nil {
		t.Fatal("data flag allowed a renderer change")
	}
}

func TestCatalogDataRevalidationRejectsChangedCompletedCases(t *testing.T) {
	root := t.TempDir()
	prior := CatalogEvidence{Schema: "test/1", CaseCount: 1, CompletedCases: 1, Mode: "deep", CatalogSHA256: "old", Batches: []string{"batch-001"}}
	raw, _ := json.Marshal(prior)
	_ = os.WriteFile(filepath.Join(root, "catalog-report.json"), raw, 0600)
	_ = os.Mkdir(filepath.Join(root, "batch-001"), 0755)
	raw, _ = json.Marshal(AtlasData{Cases: []AtlasCase{{ID: "old-case"}}})
	_ = os.WriteFile(filepath.Join(root, "batch-001", "template.json"), raw, 0600)
	expected := prior
	expected.CatalogSHA256 = "new"
	if _, err := resumeCatalogEvidence(root, expected, []AtlasCase{{ID: "changed-case"}}, newCatalogPixelGate(), io.Discard, true); err == nil {
		t.Fatal("changed completed case reused")
	}
}
