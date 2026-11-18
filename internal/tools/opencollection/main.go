// Command opencollection writes the Bruno collection in api/bruno from the
// contract, api/openapi.yaml (ADR-0008), in the OpenCollection YAML format
// that Bruno 3 opens. `make generate` runs it.
//
// The collection signs in to the instance's OpenID Connect provider with the
// authorization code flow and PKCE, and every request inherits that. There is
// one folder per tag and one request per operation, with path parameters
// taken from collection variables of the same name, a fresh Idempotency-Key
// on every POST, and a request body built from the schema's examples. A
// request that creates a resource stores the new ID in the variable that
// names it, so the next request can use it.
//
// It owns opencollection.yml and the tag folders: anything
// changed there in Bruno is overwritten on the next run.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	spec := flag.String("spec", "api/openapi.yaml", "the OpenAPI document")
	out := flag.String("out", "api/bruno", "the collection's directory")
	flag.Parse()

	if err := run(*spec, *out); err != nil {
		log.Fatalf("opencollection: %v", err)
	}
}

func run(specPath, out string) error {
	raw, err := os.ReadFile(specPath) //nolint:gosec // a path given by make generate
	if err != nil {
		return fmt.Errorf("read the contract: %w", err)
	}

	doc, err := parse(raw)
	if err != nil {
		return err
	}

	files, err := collection(doc)
	if err != nil {
		return err
	}

	if err := clean(out); err != nil {
		return err
	}

	for name, content := range files {
		path := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}

		if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // a collection checked into the repository, read by Bruno
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}

// clean removes the generated YAML files, so an operation or a tag removed
// from the contract leaves no request behind. Files it does not own, such as
// .env, .gitignore, README.md and the environments Bruno saves, stay.
func clean(out string) error {
	err := filepath.WalkDir(out, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path == filepath.Join(out, "environments") {
			return filepath.SkipDir
		}

		if !d.IsDir() && filepath.Ext(path) == ".yml" {
			return os.Remove(path) //nolint:gosec // a directory in the repository, written only by this command
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove the old collection: %w", err)
	}

	return nil
}

// The parts of the OpenAPI document the collection is built from.
type document struct {
	Info struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
	} `yaml:"info"`
	Servers []struct {
		URL string `yaml:"url"`
	} `yaml:"servers"`
	Tags []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	} `yaml:"tags"`
	Components struct {
		Parameters map[string]parameter `yaml:"parameters"`
		Schemas    map[string]*schema   `yaml:"schemas"`
	} `yaml:"components"`

	operations []operation // in the document's order
}

type operation struct {
	Path        string
	Method      string
	OperationID string `yaml:"operationId"`
	Summary     string `yaml:"summary"`
	Description string `yaml:"description"`
	Tags        []string
	Parameters  []parameter `yaml:"parameters"`
	RequestBody *struct {
		Content map[string]struct {
			Schema *schema `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"requestBody"`
	Responses map[string]struct {
		Content map[string]struct {
			Schema *schema `yaml:"schema"`
		} `yaml:"content"`
	} `yaml:"responses"`
}

type parameter struct {
	Ref         string  `yaml:"$ref"`
	Name        string  `yaml:"name"`
	In          string  `yaml:"in"`
	Required    bool    `yaml:"required"`
	Description string  `yaml:"description"`
	Schema      *schema `yaml:"schema"`
}

type schema struct {
	Ref         string             `yaml:"$ref"`
	Type        string             `yaml:"type"`
	Format      string             `yaml:"format"`
	Description string             `yaml:"description"`
	Example     any                `yaml:"example"`
	Default     any                `yaml:"default"`
	Enum        []any              `yaml:"enum"`
	Required    []string           `yaml:"required"`
	Properties  map[string]*schema `yaml:"properties"`
	Items       *schema            `yaml:"items"`
	ReadOnly    bool               `yaml:"readOnly"`

	order []string // property names in the document's order
}

func (s *schema) UnmarshalYAML(n *yaml.Node) error {
	type plain schema
	if err := n.Decode((*plain)(s)); err != nil {
		return err //nolint:wrapcheck // yaml reports the position
	}

	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value != "properties" {
			continue
		}

		props := n.Content[i+1]
		for j := 0; j < len(props.Content); j += 2 {
			s.order = append(s.order, props.Content[j].Value)
		}
	}

	return nil
}

const jsonType = "application/json"

var methods = []string{"get", "post", "put", "patch", "delete"}

func parse(raw []byte) (*document, error) {
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse the contract: %w", err)
	}

	// The paths and their operations, in the document's order.
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse the contract: %w", err)
	}

	paths := mapping(root.Content[0], "paths")
	if paths == nil {
		return nil, errors.New("the contract has no paths")
	}

	for i := 0; i < len(paths.Content); i += 2 {
		path, item := paths.Content[i].Value, paths.Content[i+1]

		for j := 0; j < len(item.Content); j += 2 {
			method := item.Content[j].Value
			if !slices.Contains(methods, method) {
				continue
			}

			op := operation{Path: path, Method: strings.ToUpper(method)}
			if err := item.Content[j+1].Decode(&op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}

			for k, p := range op.Parameters {
				if p.Ref != "" {
					resolved, ok := doc.Components.Parameters[strings.TrimPrefix(p.Ref, "#/components/parameters/")]
					if !ok {
						return nil, fmt.Errorf("%s: unknown parameter %s", op.OperationID, p.Ref)
					}

					op.Parameters[k] = resolved
				}
			}

			doc.operations = append(doc.operations, op)
		}
	}

	return &doc, nil
}

func mapping(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}

	return nil
}

func (d *document) resolve(s *schema) *schema {
	for s != nil && s.Ref != "" {
		s = d.Components.Schemas[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
	}

	return s
}

// The OpenCollection files. Field order is the order Bruno writes.

type root struct {
	OpenCollection string          `yaml:"opencollection"`
	Info           info            `yaml:"info"`
	Request        requestDefaults `yaml:"request"`
	Docs           *docs           `yaml:"docs,omitempty"`
	Bundled        bool            `yaml:"bundled"`
	Extensions     map[string]any  `yaml:"extensions"`
}

type info struct {
	Name string `yaml:"name"`
	Type string `yaml:"type,omitempty"`
	Seq  int    `yaml:"seq,omitempty"`
}

type requestDefaults struct {
	Headers   []keyValue `yaml:"headers,omitempty"`
	Auth      any        `yaml:"auth"`
	Variables []keyValue `yaml:"variables,omitempty"`
}

type docs struct {
	Content string `yaml:"content"`
	Type    string `yaml:"type"`
}

type keyValue struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
}

type oauth2 struct {
	Type             string      `yaml:"type"`
	Flow             string      `yaml:"flow"`
	AuthorizationURL string      `yaml:"authorizationUrl"`
	AccessTokenURL   string      `yaml:"accessTokenUrl"`
	RefreshTokenURL  string      `yaml:"refreshTokenUrl"`
	CallbackURL      string      `yaml:"callbackUrl"`
	Credentials      credentials `yaml:"credentials"`
	Scope            string      `yaml:"scope"`
	PKCE             struct{}    `yaml:"pkce"` // present means enabled; Bruno always uses S256
	TokenConfig      tokenConfig `yaml:"tokenConfig"`
	Settings         struct {
		AutoFetchToken   bool `yaml:"autoFetchToken"`
		AutoRefreshToken bool `yaml:"autoRefreshToken"`
	} `yaml:"settings"`
}

type credentials struct {
	ClientID  string `yaml:"clientId"`
	Placement string `yaml:"placement"`
}

type tokenConfig struct {
	ID        string `yaml:"id"`
	Placement struct {
		Header string `yaml:"header"` // the prefix of the Authorization header
	} `yaml:"placement"`
	Source string `yaml:"source"`
}

type folder struct {
	Info    info `yaml:"info"`
	Request struct {
		Auth string `yaml:"auth"`
	} `yaml:"request"`
	Docs *docs `yaml:"docs,omitempty"`
}

type request struct {
	Info    info     `yaml:"info"`
	HTTP    http     `yaml:"http"`
	Runtime *runtime `yaml:"runtime,omitempty"`
	Docs    string   `yaml:"docs,omitempty"`
}

type http struct {
	Method  string     `yaml:"method"`
	URL     string     `yaml:"url"`
	Params  []param    `yaml:"params,omitempty"`
	Headers []keyValue `yaml:"headers,omitempty"`
	Body    any        `yaml:"body,omitempty"`
	Auth    string     `yaml:"auth"`
}

type param struct {
	Name        string `yaml:"name"`
	Value       string `yaml:"value"`
	Type        string `yaml:"type"`
	Description string `yaml:"description,omitempty"`
	Disabled    bool   `yaml:"disabled,omitempty"`
}

type rawBody struct {
	Type string `yaml:"type"`
	Data string `yaml:"data"`
}

type multipartBody struct {
	Type string          `yaml:"type"`
	Data []multipartPart `yaml:"data"`
}

type multipartPart struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Value       any    `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Disabled    bool   `yaml:"disabled,omitempty"`
}

type runtime struct {
	Scripts []script `yaml:"scripts"`
}

type script struct {
	Type string `yaml:"type"`
	Code string `yaml:"code"`
}

func collection(doc *document) (map[string][]byte, error) {
	files := map[string][]byte{}
	add := func(name string, v any) error {
		b, err := encode(v)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		files[name] = b

		return nil
	}

	r := root{
		OpenCollection: "1.0.0",
		Info:           info{Name: doc.Info.Title},
		Request: requestDefaults{
			Headers:   []keyValue{{Name: "Accept", Value: "application/json"}},
			Auth:      signIn(),
			Variables: variables(doc),
		},
		Docs:       &docs{Content: collectionDocs(doc), Type: "text/markdown"},
		Extensions: map[string]any{},
	}
	if err := add("opencollection.yml", r); err != nil {
		return nil, err
	}

	ids := pathParameters(doc)
	folders := map[string]string{} // tag -> directory
	seqs := map[string]int{}

	for i, tag := range doc.Tags {
		dir := slug(tag.Name)
		folders[tag.Name] = dir

		f := folder{Info: info{Name: tag.Name, Type: "folder", Seq: i + 1}}
		f.Request.Auth = "inherit"

		if tag.Description != "" {
			f.Docs = &docs{Content: tag.Description, Type: "text/markdown"}
		}

		if err := add(dir+"/folder.yml", f); err != nil {
			return nil, err
		}
	}

	for _, op := range doc.operations {
		if len(op.Tags) == 0 {
			return nil, fmt.Errorf("%s has no tag", op.OperationID)
		}

		dir, ok := folders[op.Tags[0]]
		if !ok {
			return nil, fmt.Errorf("%s: tag %q is not declared", op.OperationID, op.Tags[0])
		}

		seqs[dir]++

		req, err := doc.request(op, seqs[dir], ids)
		if err != nil {
			return nil, err
		}

		if err := add(dir+"/"+slug(op.OperationID)+".yml", req); err != nil {
			return nil, err
		}
	}

	return files, nil
}

// signIn is the collection's authentication: the authorization code flow
// with PKCE against the provider at {{authUrl}}, as the public client
// {{clientId}}. offline_access gets a refresh token.
func signIn() oauth2 {
	a := oauth2{ //nolint:gosec // variable references, not credentials
		Type:             "oauth2",
		Flow:             "authorization_code",
		AuthorizationURL: "{{authUrl}}/oauth/v2/authorize",
		AccessTokenURL:   "{{authUrl}}/oauth/v2/token",
		RefreshTokenURL:  "{{authUrl}}/oauth/v2/token",
		CallbackURL:      "https://oauth.usebruno.com/callback",
		Credentials:      credentials{ClientID: "{{clientId}}", Placement: "body"},
		Scope:            "openid profile email offline_access",
	}
	a.TokenConfig.ID = "credentials"
	a.TokenConfig.Placement.Header = "Bearer"
	a.TokenConfig.Source = "access_token"
	a.Settings.AutoFetchToken = true
	a.Settings.AutoRefreshToken = true

	return a
}

func collectionDocs(doc *document) string {
	return strings.TrimSpace(doc.Info.Description) + `

This collection is generated from api/openapi.yaml by ` + "`make generate`" + ` (internal/tools/opencollection);
change the contract, not the requests. See README.md in this directory for signing in.`
}

// variables are the collection's variables, set for the development setup,
// so requests work without choosing an environment. An environment's
// variables take precedence over them, to point the collection elsewhere.
func variables(doc *document) []keyValue {
	// A server URL relative to the origin (ADR-0021) is taken on the
	// development app.
	const devOrigin = "http://localhost:8080"

	baseURL := devOrigin
	if len(doc.Servers) > 0 {
		baseURL = doc.Servers[0].URL
		if strings.HasPrefix(baseURL, "/") {
			baseURL = devOrigin + baseURL
		}
	}

	vars := []keyValue{
		{Name: "baseUrl", Value: baseURL, Description: "The API (make run)"},
		{Name: "authUrl", Value: "http://localhost:8081", Description: "The OpenID Connect provider, WA_OIDC_ISSUER (Zitadel from make dev-up)"},
		{
			Name: "clientId", Value: "{{process.env.WA_OAUTH_CLIENT_ID}}",
			Description: "The client ID of the Zitadel application api-clients; make dev-up writes it to .env",
		},
	}

	for _, p := range pathParameters(doc) {
		vars = append(vars, keyValue{
			Name: p.Name, Value: example(p.Schema), Description: p.Description,
		})
	}

	return vars
}

// pathParameters returns every path parameter of the contract once, in the
// order they first appear. Each is a collection variable.
func pathParameters(doc *document) []parameter {
	var params []parameter

	seen := map[string]bool{}

	for _, op := range doc.operations {
		for _, p := range op.Parameters {
			if p.In == "path" && !seen[p.Name] {
				seen[p.Name] = true

				if p.Schema != nil && p.Schema.Format == "uuid" {
					p.Schema = &schema{Type: "string"} // unknown until a request creates one
				}

				params = append(params, p)
			}
		}
	}

	return params
}

func example(s *schema) string {
	if s == nil || s.Example == nil {
		return ""
	}

	return fmt.Sprint(s.Example)
}

var pathParam = regexp.MustCompile(`\{([^}]+)\}`)

func (d *document) request(op operation, seq int, ids []parameter) (request, error) {
	req := request{
		Info: info{Name: op.Summary, Type: "http", Seq: seq},
		HTTP: http{
			Method: op.Method,
			URL:    "{{baseUrl}}" + pathParam.ReplaceAllString(op.Path, ":$1"),
			Auth:   "inherit",
		},
		Docs: strings.TrimSpace(op.Description) + "\n\nOperation `" + op.OperationID + "`.",
	}

	for _, p := range op.Parameters {
		switch p.In {
		case "path":
			req.HTTP.Params = append(req.HTTP.Params, param{
				Name: p.Name, Value: "{{" + p.Name + "}}", Type: "path", Description: p.Description,
			})
		case "query":
			value := example(p.Schema)
			if value == "" && p.Schema != nil && p.Schema.Default != nil {
				value = fmt.Sprint(p.Schema.Default)
			}

			req.HTTP.Params = append(req.HTTP.Params, param{
				Name: p.Name, Value: value, Type: "query", Description: firstLine(p.Description), Disabled: !p.Required,
			})
		case "header":
			value := ""
			if p.Name == "Idempotency-Key" {
				value = "{{$randomUUID}}"
			}

			req.HTTP.Headers = append(req.HTTP.Headers, keyValue{Name: p.Name, Value: value, Description: firstLine(p.Description)})
		default:
			return request{}, fmt.Errorf("%s: parameter %s in %s is not supported", op.OperationID, p.Name, p.In)
		}
	}

	if op.RequestBody != nil {
		body, err := d.body(op)
		if err != nil {
			return request{}, err
		}

		req.HTTP.Body = body
	}

	if code := d.storeCreatedID(op, ids); code != "" {
		req.Runtime = &runtime{Scripts: []script{{Type: "after-response", Code: code}}}
	}

	return req, nil
}

func (d *document) body(op operation) (any, error) {
	if c, ok := op.RequestBody.Content[jsonType]; ok {
		b, err := jsonExample(d.exampleValue(c.Schema, 0))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op.OperationID, err)
		}

		return rawBody{Type: "json", Data: b}, nil
	}

	if c, ok := op.RequestBody.Content["multipart/form-data"]; ok {
		s := d.resolve(c.Schema)
		body := multipartBody{Type: "multipart-form"}

		for _, name := range s.order {
			prop := d.resolve(s.Properties[name])
			part := multipartPart{
				Name: name, Type: "text", Description: firstLine(prop.Description),
				Disabled: !slices.Contains(s.Required, name),
			}

			if prop.Format == "binary" {
				part.Type, part.Value = "file", []string{}
			} else {
				part.Value = fmt.Sprint(d.exampleValue(prop, 0))
			}

			body.Data = append(body.Data, part)
		}

		return body, nil
	}

	return nil, fmt.Errorf("%s: no supported request body media type", op.OperationID)
}

// storeCreatedID returns an after-response script that keeps the ID of the
// resource a request created in the path parameter's variable: a 201 with a
// StatementImport sets statement_import_id from its statement_import_id.
func (d *document) storeCreatedID(op operation, ids []parameter) string {
	c, ok := op.Responses["201"].Content[jsonType]
	if !ok || c.Schema == nil || c.Schema.Ref == "" {
		return ""
	}

	name := strings.ReplaceAll(slug(strings.TrimPrefix(c.Schema.Ref, "#/components/schemas/")), "-", "_") + "_id"
	if _, ok := d.resolve(c.Schema).Properties[name]; !ok {
		return ""
	}

	if !slices.ContainsFunc(ids, func(p parameter) bool { return p.Name == name }) {
		return ""
	}

	return fmt.Sprintf("if (res.status === 201) {\n  bru.setVar(%q, res.body.%s);\n}", name, name)
}

const maxDepth = 6 // the Category tree is recursive

func (d *document) exampleValue(s *schema, depth int) any {
	s = d.resolve(s)
	if s == nil || depth > maxDepth {
		return nil
	}

	switch {
	case s.Example != nil:
		return s.Example
	case len(s.Enum) > 0:
		return s.Enum[0]
	case s.Default != nil:
		return s.Default
	}

	switch s.Type {
	case "object":
		obj := yaml.Node{Kind: yaml.MappingNode}

		for _, name := range s.order {
			prop := d.resolve(s.Properties[name])
			if prop.ReadOnly {
				continue
			}

			v := d.exampleValue(prop, depth+1)

			var key, val yaml.Node
			key.SetString(name)

			if err := val.Encode(v); err != nil {
				continue
			}

			obj.Content = append(obj.Content, &key, &val)
		}

		return &obj
	case "array":
		if v := d.exampleValue(s.Items, depth+1); v != nil {
			return []any{v}
		}

		return []any{}
	case "integer", "number":
		return 0
	case "boolean":
		return false
	}

	switch s.Format {
	case "uuid":
		return "00000000-0000-0000-0000-000000000000"
	case "date":
		return "2026-01-31"
	case "date-time":
		return "2026-01-31T12:00:00Z"
	}

	return "string"
}

// jsonExample writes v as indented JSON, keeping the schema's property order.
func jsonExample(v any) (string, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return "", fmt.Errorf("example: %w", err)
	}

	var b strings.Builder

	writeJSON(&b, &n, "")

	return b.String(), nil
}

func writeJSON(b *strings.Builder, n *yaml.Node, indent string) {
	switch n.Kind {
	case yaml.DocumentNode:
		writeJSON(b, n.Content[0], indent)
	case yaml.MappingNode:
		if len(n.Content) == 0 {
			b.WriteString("{}")

			return
		}

		b.WriteString("{\n")

		for i := 0; i < len(n.Content); i += 2 {
			fmt.Fprintf(b, "%s  %q: ", indent, n.Content[i].Value)
			writeJSON(b, n.Content[i+1], indent+"  ")

			if i+2 < len(n.Content) {
				b.WriteString(",")
			}

			b.WriteString("\n")
		}

		b.WriteString(indent + "}")
	case yaml.SequenceNode:
		if len(n.Content) == 0 {
			b.WriteString("[]")

			return
		}

		b.WriteString("[\n")

		for i, c := range n.Content {
			b.WriteString(indent + "  ")
			writeJSON(b, c, indent+"  ")

			if i+1 < len(n.Content) {
				b.WriteString(",")
			}

			b.WriteString("\n")
		}

		b.WriteString(indent + "]")
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!int", "!!float", "!!bool":
			b.WriteString(n.Value)
		case "!!null":
			b.WriteString("null")
		default:
			fmt.Fprintf(b, "%q", n.Value)
		}
	case yaml.AliasNode:
		writeJSON(b, n.Alias, indent)
	}
}

func encode(v any) ([]byte, error) {
	var b bytes.Buffer

	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)

	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	return b.Bytes(), nil
}

var (
	nonWord       = regexp.MustCompile(`[^a-z0-9]+`)
	camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
)

func slug(s string) string {
	s = camelBoundary.ReplaceAllString(s, "$1-$2")

	return strings.Trim(nonWord.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// firstLine returns the first sentence of a description, on one line, for a
// table cell in Bruno.
func firstLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}

	return s
}
