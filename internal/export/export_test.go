package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justfun/logview/internal/model"
)

func TestExportRaw(t *testing.T) {
	lines := []*model.ParsedLine{
		{Raw: model.RawLine{Text: "line1"}, Fields: map[model.Field]string{model.FieldMessage: "line1"}},
		{Raw: model.RawLine{Text: "line2"}, Fields: map[model.Field]string{model.FieldMessage: "line2"}},
	}

	dir := t.TempDir()
	fpath := filepath.Join(dir, "export.log")
	n, err := ToFile(lines, fpath, FormatRaw)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("exported %d lines, want 2", n)
	}

	data, _ := os.ReadFile(fpath)
	content := string(data)
	if !strings.Contains(content, "line1\n") || !strings.Contains(content, "line2\n") {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestExportJSON(t *testing.T) {
	lines := []*model.ParsedLine{
		{Fields: map[model.Field]string{model.FieldLevel: "INFO", model.FieldMessage: "hello"}},
	}

	dir := t.TempDir()
	fpath := filepath.Join(dir, "export.json")
	n, err := ToFile(lines, fpath, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("exported %d lines, want 1", n)
	}

	data, _ := os.ReadFile(fpath)
	content := string(data)
	if !strings.Contains(content, `"level":"INFO"`) || !strings.Contains(content, `"message":"hello"`) {
		t.Errorf("unexpected JSON: %q", content)
	}
}