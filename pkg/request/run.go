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
	"net/url"
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
	Host         string
	Iteration    int
	Page         int
	PageSize     int
	ResultOffset int
	AuthToken    string
	Extra        map[string]interface{}
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

func (tr *TemplateRequest) ShouldContinue(resp *http.Response, body []byte) bool {
	if tr.StopWhen == nil || len(tr.StopWhen) == 0 {
		// no conditions. do not continue
		return false
	}

	sr := SimpleResponse{
		Request: SimpleRequest{
			Path:  resp.Request.URL.Path,
			Query: resp.Request.URL.Query(),
		},
		Status:      resp.StatusCode,
		RawBody:     string(body),
		ContentType: resp.Header.Get("Content-Type"),
		Headers:     resp.Header,
	}

	maybe := map[string]any{}
	json.Unmarshal(body, &maybe)

	maybeNot := []map[string]any{}
	json.Unmarshal(body, &maybeNot)

	sr.BodyObject = maybe
	sr.BodyArray = maybeNot

	jsonM, err := json.Marshal(sr)
	if err != nil {
		log.Println("error marshalling json of simple request", err)
		panic(err)
	}
	r := map[string]any{}
	err = json.Unmarshal(jsonM, &r)
	if err != nil {
		log.Println("error unmarshalling json of simple request", err)
		panic(err)
	}

	for _, condition := range tr.StopWhen {
		query, err := gojq.Parse(condition)
		if err != nil {
			log.Println(err)
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

				log.Println(err)
			}

			if v != nil {
				return false
			}
		}
	}

	// no matches, continue
	return true
}

type SimpleRequest struct {
	Path  string     `json:"path"`
	Query url.Values `json:"query"`
}

type SimpleResponse struct {
	Request     SimpleRequest       `json:"request"`
	Status      int                 `json:"status"`
	RawBody     string              `json:"raw_body"`
	BodyObject  any                 `json:"body_object"`
	BodyArray   any                 `json:"body_array"`
	ContentType string              `json:"content_type"`
	Headers     map[string][]string `json:"headers"`
}

func (tr *TemplateRequest) Recurse(c *RequestContext, handleResponse func(body []byte)) {
	for reqCount := 0; true; reqCount++ {
		c.Iteration = reqCount
		c.Page = reqCount + 1
		c.ResultOffset = c.PageSize * reqCount

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

		if !tr.ShouldContinue(resp, body) {
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
