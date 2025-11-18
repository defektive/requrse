package request

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/gorilla/websocket"
	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"
)

type HeaderTemplate struct {
	HeaderTemplate *template.Template
	ValueTemplate  *template.Template
}

type TemplateRequest struct {
	Name      string            `yaml:"name"`
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
	SetupBody string            `yaml:"setup_body"`
	Body      string            `yaml:"body"`
	Method    string            `yaml:"method"`
	StopWhen  []string          `yaml:"stop_when"`
	Lists     [][]string        `yaml:"lists"`

	headerTemplates map[string]*HeaderTemplate
	bodyTemplate    *template.Template
	urlTemplate     *template.Template

	LastResponse SimpleResponse

	webSocket *websocket.Conn

	proxyURL *url.URL
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

func (tr *TemplateRequest) SetProxy(proxyString string) error {
	parsed, err := url.Parse(proxyString)
	if err != nil {
		return err
	}
	tr.proxyURL = parsed

	return nil
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
	ListParams   []string
	LastResponse *SimpleResponse
}

func (tr *TemplateRequest) Send(c *RequestContext) ([]byte, bool, error) {

	var bodyBytes bytes.Buffer
	tr.BodyTemplate().Execute(&bodyBytes, c)

	var urlBytes bytes.Buffer
	tr.URLTemplate().Execute(&urlBytes, c)

	requestURL := urlBytes.String()
	httpHeader := http.Header{}

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

		httpHeader.Set(hdrBytes.String(), valBytes.String())
	}

	if strings.HasPrefix(requestURL, "http") {
		// we are working HTTP

		req, err := http.NewRequest(tr.Method, requestURL, &bodyBytes)
		if err != nil {
			return nil, false, err
		}
		req.Header = httpHeader
		client := &http.Client{}

		if tr.proxyURL != nil {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: true, // This disables certificate verification
			}

			proxy := http.ProxyURL(tr.proxyURL)
			transport := &http.Transport{
				Proxy:           proxy,
				TLSClientConfig: tlsConfig,
			}
			client.Transport = transport
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, false, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, err
		}

		shouldContinue := tr.ShouldContinueHTTP(resp, body)
		return body, shouldContinue, nil
	} else if strings.HasPrefix(requestURL, "ws:") {
		// we are working with websockets!!
		//parsedProxy, err := url.Parse("http://127.0.0.1:8080")
		//websocket.DefaultDialer.Proxy = http.ProxyURL(parsedProxy)
		ws := tr.getWS(requestURL, httpHeader)

		if c.Iteration == 0 && tr.SetupBody != "" {
			// hack to test if this could be useful
			if err := ws.WriteMessage(websocket.TextMessage, []byte(tr.SetupBody)); err != nil {
				return nil, false, err
			}

			if _, msg, err := ws.ReadMessage(); err != nil {
				log.Fatal(err)
			} else {
				log.Println(string(msg))
			}
		}

		if err := ws.WriteMessage(websocket.TextMessage, bodyBytes.Bytes()); err != nil {
			return nil, false, err
		}

		if _, msg, err := ws.ReadMessage(); err != nil {
			log.Fatal(err)
		} else {
			shouldContinue := tr.ShouldContinueWS(msg)
			return msg, shouldContinue, nil
		}
	}

	return nil, false, errors.New("invalid request")
}

func (tr *TemplateRequest) getWS(requestURL string, httpHeader http.Header) *websocket.Conn {
	if tr.webSocket == nil {
		ws, _, err := websocket.DefaultDialer.Dial(requestURL, httpHeader)
		if err != nil {
			panic(err)
		}
		tr.webSocket = ws
	}
	return tr.webSocket
}

func (tr *TemplateRequest) ShouldContinueHTTP(resp *http.Response, body []byte) bool {
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

	tr.LastResponse = sr

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
			}

			if v != nil {
				return false
			}
		}
	}

	// no matches, continue
	return true
}

func (tr *TemplateRequest) ShouldContinueWS(body []byte) bool {
	if tr.StopWhen == nil || len(tr.StopWhen) == 0 {
		// no conditions. do not continue
		return false
	}

	sr := SimpleResponse{
		RawBody: string(body),
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
		c.LastResponse = &tr.LastResponse

		if len(tr.Lists) > 0 {
			c.ListParams = []string{}
			for _, list := range tr.Lists {
				if val := list[reqCount]; val != "" {
					c.ListParams = append(c.ListParams, val)
				} else {
					log.Printf("list[%d] is empty", reqCount)
				}
			}
		}

		body, shouldContinue, err := tr.Send(c)
		if err != nil {
			panic(err)
		}

		handleResponse(body)

		if !shouldContinue {
			return
		}
	}
}

func FromFile(filename string) (*TemplateRequest, error) {
	f, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return FromBytes(f)
}

func FromBytes(fileByes []byte) (*TemplateRequest, error) {
	var request *TemplateRequest
	err := yaml.Unmarshal(fileByes, &request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

type PayloadData struct {
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}
