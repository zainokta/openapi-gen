package spec

// OpenAPISpec represents the OpenAPI 3.0 specification
type OpenAPISpec struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]PathItem   `json:"paths"`
	Components Components            `json:"components,omitempty"`
	Security   []SecurityRequirement `json:"security,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
}

type Info struct {
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Version     string  `json:"version"`
	Contact     Contact `json:"contact,omitempty"`
}

type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type Server struct {
	URL         string                    `json:"url"`
	Description string                    `json:"description,omitempty"`
	Variables   map[string]ServerVariable `json:"variables,omitempty"`
}

type ServerVariable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description,omitempty"`
}

type PathItem struct {
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Get         *Operation  `json:"get,omitempty"`
	Put         *Operation  `json:"put,omitempty"`
	Post        *Operation  `json:"post,omitempty"`
	Delete      *Operation  `json:"delete,omitempty"`
	Options     *Operation  `json:"options,omitempty"`
	Head        *Operation  `json:"head,omitempty"`
	Patch       *Operation  `json:"patch,omitempty"`
	Trace       *Operation  `json:"trace,omitempty"`
	Parameters  []Parameter `json:"parameters,omitempty"`
}

type Operation struct {
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses,omitempty"`
	Deprecated  bool                  `json:"deprecated,omitempty"`
	Security    []SecurityRequirement `json:"security,omitempty"`
}

type Parameter struct {
	Name            string             `json:"name"`
	In              string             `json:"in"` // query, header, path, cookie
	Description     string             `json:"description,omitempty"`
	Required        bool               `json:"required,omitempty"`
	Deprecated      bool               `json:"deprecated,omitempty"`
	AllowEmptyValue bool               `json:"allowEmptyValue,omitempty"`
	Style           string             `json:"style,omitempty"`
	Explode         bool               `json:"explode,omitempty"`
	AllowReserved   bool               `json:"allowReserved,omitempty"`
	Schema          Schema             `json:"schema,omitempty"`
	Example         interface{}        `json:"example,omitempty"`
	Examples        map[string]Example `json:"examples,omitempty"`
}

type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Required    bool                 `json:"required,omitempty"`
}

type MediaType struct {
	Schema   Schema              `json:"schema,omitempty"`
	Example  interface{}         `json:"example,omitempty"`
	Examples map[string]Example  `json:"examples,omitempty"`
	Encoding map[string]Encoding `json:"encoding,omitempty"`
}

type Encoding struct {
	ContentType   string            `json:"contentType,omitempty"`
	Headers       map[string]Header `json:"headers,omitempty"`
	Style         string            `json:"style,omitempty"`
	Explode       bool              `json:"explode,omitempty"`
	AllowReserved bool              `json:"allowReserved,omitempty"`
}

type Response struct {
	Description string               `json:"description"`
	Headers     map[string]Header    `json:"headers,omitempty"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Links       map[string]Link      `json:"links,omitempty"`
}

type Header struct {
	Description     string             `json:"description,omitempty"`
	Required        bool               `json:"required,omitempty"`
	Deprecated      bool               `json:"deprecated,omitempty"`
	AllowEmptyValue bool               `json:"allowEmptyValue,omitempty"`
	Style           string             `json:"style,omitempty"`
	Explode         bool               `json:"explode,omitempty"`
	AllowReserved   bool               `json:"allowReserved,omitempty"`
	Schema          Schema             `json:"schema,omitempty"`
	Example         interface{}        `json:"example,omitempty"`
	Examples        map[string]Example `json:"examples,omitempty"`
}

type Link struct {
	OperationRef string                 `json:"operationRef,omitempty"`
	OperationID  string                 `json:"operationId,omitempty"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	RequestBody  interface{}            `json:"requestBody,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Server       Server                 `json:"server,omitempty"`
}

type Example struct {
	Summary       string      `json:"summary,omitempty"`
	Description   string      `json:"description,omitempty"`
	Value         interface{} `json:"value,omitempty"`
	ExternalValue string      `json:"externalValue,omitempty"`
}

type Components struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty"`
	Responses       map[string]Response       `json:"responses,omitempty"`
	Parameters      map[string]Parameter      `json:"parameters,omitempty"`
	Examples        map[string]Example        `json:"examples,omitempty"`
	RequestBodies   map[string]RequestBody    `json:"requestBodies,omitempty"`
	Headers         map[string]Header         `json:"headers,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
	Links           map[string]Link           `json:"links,omitempty"`
	Callbacks       map[string]Callback       `json:"callbacks,omitempty"`
}

type Schema struct {
	Type                 string            `json:"type,omitempty"`
	AllOf                []Schema          `json:"allOf,omitempty"`
	OneOf                []Schema          `json:"oneOf,omitempty"`
	AnyOf                []Schema          `json:"anyOf,omitempty"`
	Not                  *Schema           `json:"not,omitempty"`   // Pointer for circular reference
	Items                *Schema           `json:"items,omitempty"` // Pointer for circular reference
	Properties           map[string]Schema `json:"properties,omitempty"`
	AdditionalProperties *Schema           `json:"additionalProperties,omitempty"` // Pointer for circular reference
	Description          string            `json:"description,omitempty"`
	Format               string            `json:"format,omitempty"`
	Default              interface{}       `json:"default,omitempty"`
	Example              interface{}       `json:"example,omitempty"`

	// String validation
	MaxLength *int     `json:"maxLength,omitempty"` // Pointer to distinguish 0 from nil
	MinLength *int     `json:"minLength,omitempty"` // Pointer to distinguish 0 from nil
	Pattern   string   `json:"pattern,omitempty"`
	Enum      []string `json:"enum,omitempty"`

	// Number validation
	MultipleOf       *float64 `json:"multipleOf,omitempty"` // Pointer to distinguish 0 from nil
	Maximum          *float64 `json:"maximum,omitempty"`    // Pointer to distinguish 0 from nil
	ExclusiveMaximum bool     `json:"exclusiveMaximum,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"` // Pointer to distinguish 0 from nil
	ExclusiveMinimum bool     `json:"exclusiveMinimum,omitempty"`

	// Array validation
	MaxItems    *int `json:"maxItems,omitempty"` // Pointer to distinguish 0 from nil
	MinItems    *int `json:"minItems,omitempty"` // Pointer to distinguish 0 from nil
	UniqueItems bool `json:"uniqueItems,omitempty"`

	// Object validation
	MaxProperties *int     `json:"maxProperties,omitempty"` // Pointer to distinguish 0 from nil
	MinProperties *int     `json:"minProperties,omitempty"` // Pointer to distinguish 0 from nil
	Required      []string `json:"required,omitempty"`

	// Generic validation
	Title      string `json:"title,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty"`
	WriteOnly  bool   `json:"writeOnly,omitempty"`
	Deprecated bool   `json:"deprecated,omitempty"`
	Nullable   bool   `json:"nullable,omitempty"`

	// Reference
	Ref string `json:"$ref,omitempty"`
}

type SecurityScheme struct {
	Type             string     `json:"type"`
	Description      string     `json:"description,omitempty"`
	Name             string     `json:"name,omitempty"`
	In               string     `json:"in,omitempty"`
	Scheme           string     `json:"scheme,omitempty"`
	BearerFormat     string     `json:"bearerFormat,omitempty"`
	Flows            OAuthFlows `json:"flows,omitempty"`
	OpenIDConnectURL string     `json:"openIdConnectUrl,omitempty"`
}

type OAuthFlows struct {
	Implicit          OAuthFlow `json:"implicit,omitempty"`
	Password          OAuthFlow `json:"password,omitempty"`
	ClientCredentials OAuthFlow `json:"clientCredentials,omitempty"`
	AuthorizationCode OAuthFlow `json:"authorizationCode,omitempty"`
}

type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl,omitempty"`
	TokenURL         string            `json:"tokenUrl,omitempty"`
	RefreshURL       string            `json:"refreshUrl,omitempty"`
	Scopes           map[string]string `json:"scopes"`
}

type SecurityRequirement map[string][]string

type Tag struct {
	Name         string       `json:"name"`
	Description  string       `json:"description,omitempty"`
	ExternalDocs ExternalDocs `json:"externalDocs,omitempty"`
}

type ExternalDocs struct {
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
}

type Callback map[string]PathItem

// RouteInfo holds information about a route for OpenAPI generation
type RouteInfo struct {
	Method       string
	Path         string
	HandlerName  string
	Handler      interface{}
	RequestType  interface{}
	ResponseType interface{}
	Tags         []string
	Summary      string
	Description  string
	Deprecated   bool
}
