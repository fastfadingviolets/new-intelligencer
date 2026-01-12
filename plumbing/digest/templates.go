package main

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
	"sync"
)

//go:embed templates/*.tmpl templates/*.css templates/*.js
var templateFS embed.FS

var (
	digestTemplate *template.Template
	templateOnce   sync.Once
	templateErr    error
)

// getDigestTemplate returns the parsed digest template, loading it on first call.
func getDigestTemplate() (*template.Template, error) {
	templateOnce.Do(func() {
		digestTemplate, templateErr = loadDigestTemplate()
	})
	return digestTemplate, templateErr
}

// loadDigestTemplate loads and parses all templates with the function map.
func loadDigestTemplate() (*template.Template, error) {
	// Read CSS and JS for inlining
	cssBytes, err := templateFS.ReadFile("templates/styles.css")
	if err != nil {
		return nil, fmt.Errorf("reading CSS: %w", err)
	}
	jsBytes, err := templateFS.ReadFile("templates/scripts.js")
	if err != nil {
		return nil, fmt.Errorf("reading JS: %w", err)
	}

	// Create function map
	funcs := template.FuncMap{
		"inlineCSS": func() template.CSS {
			return template.CSS(cssBytes)
		},
		"inlineJS": func() template.JS {
			return template.JS(jsBytes)
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Parse all templates
	tmpl, err := template.New("digest").Funcs(funcs).ParseFS(templateFS, "templates/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return tmpl, nil
}

// RenderDigest renders the digest HTML using templates.
func RenderDigest(data *DigestData) (string, error) {
	tmpl, err := getDigestTemplate()
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "digest.html.tmpl", data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}
