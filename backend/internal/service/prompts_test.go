package service

import (
	"strings"
	"testing"
)

func TestConvertYouMindPrompt(t *testing.T) {
	prompt, variables := convertYouMindPrompt(`A {argument name="product name" default="coffee maker"} on {argument name="surface"}`)
	if prompt != "A {{product_name}} on {{surface}}" {
		t.Fatalf("unexpected prompt: %s", prompt)
	}
	if len(variables) != 2 || variables[0].Name != "product_name" || variables[0].Default != "coffee maker" || variables[0].Required {
		t.Fatalf("unexpected variables: %#v", variables)
	}
	if !variables[1].Required {
		t.Fatal("variable without default should be required")
	}
}

func TestValidatePromptTemplateInput(t *testing.T) {
	input := PromptTemplateInput{
		CategoryID: "pc-test", Title: "Test", MediaType: "image", Status: "published",
		Prompt: "A {{ subject }}", Variables: []PromptVariable{{Name: "subject", Label: "Subject", Required: true}},
		ReferenceMode: "none", MinReferences: 2, MaxReferences: 3,
	}
	if err := validateTemplateInput(&input); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	if input.MinReferences != 0 || input.MaxReferences != 0 {
		t.Fatal("none reference mode should clear reference limits")
	}
	input.Variables[0].Name = "bad name"
	if err := validateTemplateInput(&input); err == nil || !strings.Contains(err.Error(), "变量") {
		t.Fatalf("invalid variable name accepted: %v", err)
	}
}
