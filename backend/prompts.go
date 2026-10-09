package main

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode"
)

type Prompt struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	SVG     string   `json:"svg"`
}

//go:embed prompts.json
var promptsJSON []byte

var prompts = mustLoadPrompts()

func mustLoadPrompts() []Prompt {
	var list []Prompt
	if err := json.Unmarshal(promptsJSON, &list); err != nil {
		panic(err)
	}
	if len(list) < 100 {
		panic("題庫至少需要 100 題")
	}
	return list
}

// normalize 讓「車子 !」「車子」「ＣＡＲ」之類的寫法都比對得上。
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 0xFF01 && r <= 0xFF5E {
			r = unicode.ToLower(r - 0xFEE0)
		}
		if r == '臺' {
			r = '台'
		}
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (p Prompt) matches(guess string) bool {
	g := normalize(guess)
	if g == "" {
		return false
	}
	if g == normalize(p.Name) {
		return true
	}
	for _, alias := range p.Aliases {
		if g == normalize(alias) {
			return true
		}
	}
	return false
}
