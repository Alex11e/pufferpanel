package hu_HU

import (
	"encoding/json"
	"html/template"
	"testing"
)

func TestEmailTemplatesParse(t *testing.T) {
	contents, err := Emails.ReadFile("emails.json")
	if err != nil {
		t.Fatal(err)
	}
	var declarations map[string]struct {
		Subject      string `json:"subject"`
		BodyTemplate string `json:"bodyTemplate"`
	}
	if err = json.Unmarshal(contents, &declarations); err != nil {
		t.Fatal(err)
	}
	if len(declarations) == 0 {
		t.Fatal("expected email templates")
	}
	for name, declaration := range declarations {
		if _, err = template.New(name + "-subject").Parse(declaration.Subject); err != nil {
			t.Errorf("template %s subject: %v", name, err)
		}
		if _, err = template.New(name + "-body").Parse(declaration.BodyTemplate); err != nil {
			t.Errorf("template %s body: %v", name, err)
		}
	}
}
