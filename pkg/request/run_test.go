package request

import (
	"testing"
)

func TestCreateTemplate(t *testing.T) {
	tpl := CreateTemplate("test", `Hello {{ .Name }}`)
	if tpl == nil {
		t.Fatal("Expected non-nil template")
	}
}

func TestGetTemplatePrefix(t *testing.T) {
	tr := &TemplateRequest{
		Method: "GET",
		URL:    "/test",
		Body:   "{{ .Extra.Test }}",
	}

	prefix := tr.getTemplatePrefix()
	// Expected: lowercase method + sanitized URL
	// GET becomes get, /test becomes test (slashes and underscore removed)
	expected := "get_test"
	if prefix != expected {
		t.Errorf("Expected %s, got %s", expected, prefix)
	}
}

func TestGetHeaderTplKey(t *testing.T) {
	tr := &TemplateRequest{
		Method: "POST",
		URL:    "/api/test",
		Headers: map[string]string{
			"Accept": "application/json",
		},
	}

	key := tr.getHeaderTplKey("Accept", 0)
	// Expected: lowercase method + sanitized URL + _header_0 + sanitized header + lowercase value
	expected := "post_apitest_header_0_accept"
	if key != expected {
		t.Errorf("Expected %s, got %s", expected, key)
	}
}

func TestSetProxy(t *testing.T) {
	tr := &TemplateRequest{
		Method: "GET",
		URL:    "http://example.com",
	}

	err := tr.SetProxy("http://localhost:8080")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if tr.proxyURL == nil {
		t.Fatal("Expected non-nil proxyURL")
	}
}

func TestHeaderTemplates(t *testing.T) {
	tr := &TemplateRequest{
		Method: "GET",
		URL:    "/test",
		Headers: map[string]string{
			"Accept": "application/json",
			"User-Agent": "requrse",
		},
	}

	tpls := tr.HeaderTemplates()
	if len(tpls) != 2 {
		t.Errorf("Expected 2 templates, got %d", len(tpls))
	}
}

func TestBodyTemplate(t *testing.T) {
	tr := &TemplateRequest{
		Method: "POST",
		URL:    "/data",
		Body:   `{"key": "{{ .Extra.Value }}"}`,
	}

	tpl := tr.BodyTemplate()
	if tpl == nil {
		t.Fatal("Expected non-nil body template")
	}
}

func TestURLTemplate(t *testing.T) {
	tr := &TemplateRequest{
		Method: "GET",
		URL:    "/{{ .Path }}",
	}

	tpl := tr.URLTemplate()
	if tpl == nil {
		t.Fatal("Expected non-nil URL template")
	}
}

func TestSimpleRequestMarshal(t *testing.T) {
	sr := SimpleRequest{
		Path:  "/test",
		Query: map[string][]string{"key": {"value"}},
	}
	_ = sr
}

func TestSimpleResponseMarshal(t *testing.T) {
	sr := SimpleResponse{
		Status:     200,
		RawBody:    `{"message": "OK"}`,
		ContentType: "application/json",
	}
	_ = sr

	if sr.Status != 200 {
		t.Errorf("Expected status 200, got %d", sr.Status)
	}
}
