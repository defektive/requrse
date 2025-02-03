package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
)

type HeaderTemplate struct {
	HeaderTemplate *template.Template
	ValueTemplate  *template.Template
}

type TemplateRequest struct {
	Name     string            `yaml:"name"`
	URL      string            `yaml:"url"`
	Headers  map[string]string `yaml:"headers"`
	Body     string            `yaml:"body"`
	Method   string            `yaml:"method"`
	StopWhen []string          `yaml:"stop_when"`

	headerTemplates map[string]*HeaderTemplate
	bodyTemplate    *template.Template
	urlTemplate     *template.Template
}

func CreateTemplate(name, t string) *template.Template {
	return template.Must(template.New(name).Parse(t))
}

var sanitizeRegExp = regexp.MustCompile("[^a-zA-Z0-9_-]")

func (tr *TemplateRequest) getTemplatePrefix() string {
	return sanitizeRegExp.ReplaceAllString(strings.ToLower(fmt.Sprintf("%s_%s", tr.Method, tr.URL)), "")
}

func (tr *TemplateRequest) getHeaderTplKey(header string, index int) string {
	normalizedHeader := sanitizeRegExp.ReplaceAllString(strings.ToLower(header), "")
	return fmt.Sprintf("%s_header_%d_%s", tr.getTemplatePrefix(), index, normalizedHeader)
}

func (tr *TemplateRequest) HeaderTemplates() map[string]*HeaderTemplate {
	if tr.headerTemplates == nil {
		tr.headerTemplates = make(map[string]*HeaderTemplate)
	}

	i := 0
	for header, value := range tr.Headers {
		headerHeaderTplKey := tr.getHeaderTplKey(header, i)
		if _, ok := tr.headerTemplates[headerHeaderTplKey]; !ok {
			tr.headerTemplates[headerHeaderTplKey] = &HeaderTemplate{
				HeaderTemplate: CreateTemplate(headerHeaderTplKey, header),
				ValueTemplate:  CreateTemplate(fmt.Sprintf("%s_value", headerHeaderTplKey), value),
			}
		}
	}

	return tr.headerTemplates

}

func (tr *TemplateRequest) BodyTemplate() *template.Template {
	if tr.bodyTemplate == nil {
		tr.bodyTemplate = CreateTemplate(fmt.Sprintf("%s_body", tr.getTemplatePrefix()), tr.Body)
	}

	return tr.bodyTemplate
}

func (tr *TemplateRequest) URLTemplate() *template.Template {
	if tr.urlTemplate == nil {
		tr.urlTemplate = CreateTemplate(fmt.Sprintf("%s_url", tr.getTemplatePrefix()), tr.URL)
	}

	return tr.urlTemplate
}

type RequestContext struct {
	Host      string
	Page      int
	PageSize  int
	AuthToken string
	Extra     map[string]interface{}
}

func (tr *TemplateRequest) NewRequest(c *RequestContext) (*http.Request, error) {

	var bodyBytes bytes.Buffer
	tr.BodyTemplate().Execute(&bodyBytes, c)

	var urlBytes bytes.Buffer
	tr.URLTemplate().Execute(&urlBytes, c)

	req, err := http.NewRequest(tr.Method, urlBytes.String(), &bodyBytes)
	if err != nil {
		return nil, err
	}

	for _, headerTpl := range tr.HeaderTemplates() {
		var hdrBytes bytes.Buffer
		var valBytes bytes.Buffer
		err := headerTpl.HeaderTemplate.Execute(&hdrBytes, c)
		if err != nil {
			panic(err)
		}
		err = headerTpl.ValueTemplate.Execute(&valBytes, c)
		if err != nil {
			panic(err)
		}

		req.Header.Set(hdrBytes.String(), valBytes.String())
	}

	return req, nil
}

func (tr *TemplateRequest) ShouldContinue(body []byte) bool {
	if tr.StopWhen == nil || len(tr.StopWhen) == 0 {
		// no conditions. do not continue
		return false
	}
	r := map[string]any{}
	err := json.Unmarshal(body, &r)
	if err != nil {
		panic(err)
	}

	for _, condition := range tr.StopWhen {
		query, err := gojq.Parse(condition)
		if err != nil {
			panic(err)
		}

		iter := query.Run(r)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				if err, ok := err.(*gojq.HaltError); ok && err.Value() == nil {
					break
				}
				log.Fatalln(err)
			}
			return false
		}
	}

	// no matches, continue
	return true
}

func (tr *TemplateRequest) Recurse(c *RequestContext, handleResponse func(body []byte)) {
	for reqCount := 1; reqCount <= 50; reqCount++ {
		c.Page = reqCount
		c.PageSize = 50

		req, err := tr.NewRequest(c)
		if err != nil {
			panic(err)
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			panic(err)
		}

		handleResponse(body)
		if !tr.ShouldContinue(body) {
			return
		}
	}
}

func FromFile(filename string) (*TemplateRequest, error) {
	f, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var request *TemplateRequest
	err = yaml.Unmarshal(f, &request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

type PayloadData struct {
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}
