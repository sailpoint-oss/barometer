// Package testserver provides a comprehensive Huma-based HTTP server that serves
// an auto-generated OpenAPI spec and implements operations for E2E contract testing.
package testserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// StartTestServer starts an HTTP server with a comprehensive Huma API for E2E tests.
// Returns baseURL, specURL (baseURL + /openapi.json), and a cleanup function.
func StartTestServer(t *testing.T) (baseURL, specURL string, cleanup func()) {
	t.Helper()
	router := chi.NewMux()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Request-Id", "123e4567-e89b-12d3-a456-426614174000")
			next.ServeHTTP(w, r)
		})
	})
	cfg := huma.DefaultConfig("Barometer Test API", "1.0.0")
	api := humachi.New(router, cfg)

	registerAuth(api)
	registerWidgets(api)
	registerUsers(api)
	registerArrays(api)
	registerObjects(api)
	registerParams(api)
	registerMisc(api)
	registerCompositions(api)
	registerDiscriminator(api)
	registerRecursive(api)

	srv := httptest.NewServer(router)
	return srv.URL, srv.URL + "/openapi.json", srv.Close
}

// --- Auth ---

type LoginInput struct {
	Body struct {
		Username string `json:"username" required:"true"`
		Password string `json:"password" required:"true"`
	} `json:"body"`
}

type LoginOutput struct {
	Body struct {
		Token string `json:"token"`
	} `json:"body"`
}

func registerAuth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "login",
		Method:        http.MethodPost,
		Path:          "/auth/login",
		Summary:       "Login and get token",
		DefaultStatus: 200,
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		out := &LoginOutput{}
		out.Body.Token = "Bearer test-token-" + input.Body.Username
		return out, nil
	})
}

// --- Widgets ---

type Widget struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Active bool    `json:"active"`
}

type WidgetListOutput struct {
	Body []Widget `json:"body"`
}

type CreateWidgetInput struct {
	Body struct {
		Name   string  `json:"name" minLength:"1" maxLength:"100" required:"true"`
		Weight float64 `json:"weight" minimum:"0" maximum:"1000"`
		Active bool    `json:"active"`
	} `json:"body"`
}

type CreateWidgetOutput struct {
	Status int `json:"status"`
	Body   struct {
		ID     int     `json:"id"`
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
		Active bool    `json:"active"`
	} `json:"body"`
}

type GetWidgetInput struct {
	WidgetID string `path:"widgetId" doc:"Widget ID"`
}

type GetWidgetOutput struct {
	Status int `json:"status"`
	Body   Widget
}

type DeleteWidgetInput struct {
	WidgetID string `path:"widgetId"`
}

func registerWidgets(api huma.API) {
	huma.Get(api, "/widgets", func(ctx context.Context, input *struct{}) (*WidgetListOutput, error) {
		return &WidgetListOutput{
			Body: []Widget{
				{ID: 1, Name: "A", Weight: 1.5, Active: true},
				{ID: 2, Name: "B", Weight: 2.0, Active: false},
			},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "createWidget",
		Method:        http.MethodPost,
		Path:          "/widgets",
		Summary:       "Create widget",
		DefaultStatus: 201,
	}, func(ctx context.Context, input *CreateWidgetInput) (*CreateWidgetOutput, error) {
		out := &CreateWidgetOutput{Status: 201}
		out.Body.ID = 99
		out.Body.Name = input.Body.Name
		out.Body.Weight = input.Body.Weight
		out.Body.Active = input.Body.Active
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "getWidget",
		Method:        http.MethodGet,
		Path:          "/widgets/{widgetId}",
		Summary:       "Get widget by ID",
		DefaultStatus: 200,
	}, func(ctx context.Context, input *GetWidgetInput) (*GetWidgetOutput, error) {
		if input.WidgetID == "missing" {
			return nil, huma.Error404NotFound("widget not found")
		}
		return &GetWidgetOutput{
			Status: 200,
			Body:   Widget{ID: 1, Name: "A", Weight: 1.5, Active: true},
		}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "deleteWidget",
		Method:        http.MethodDelete,
		Path:          "/widgets/{widgetId}",
		Summary:       "Delete widget",
		DefaultStatus: 204,
	}, func(ctx context.Context, input *DeleteWidgetInput) (*struct{}, error) {
		return nil, nil
	})
}

// --- Users (readOnly, writeOnly, formats) ---

type CreateUserInput struct {
	Body struct {
		Email    string `json:"email" format:"email" required:"true"`
		Password string `json:"password" writeOnly:"true" required:"true"`
	} `json:"body"`
}

type UserOutput struct {
	Body struct {
		ID        string `json:"id" format:"uuid" readOnly:"true"`
		Email     string `json:"email" format:"email"`
		CreatedAt string `json:"createdAt" format:"date-time" readOnly:"true"`
	} `json:"body"`
}

type GetUserInput struct {
	UserID string `path:"userId"`
}

func registerUsers(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "createUser",
		Method:        http.MethodPost,
		Path:          "/users",
		Summary:       "Create user",
		DefaultStatus: 201,
	}, func(ctx context.Context, input *CreateUserInput) (*UserOutput, error) {
		out := &UserOutput{}
		out.Body.ID = "550e8400-e29b-41d4-a716-446655440000"
		out.Body.Email = input.Body.Email
		out.Body.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		return out, nil
	})

	huma.Get(api, "/users/{userId}", func(ctx context.Context, input *GetUserInput) (*UserOutput, error) {
		out := &UserOutput{}
		out.Body.ID = input.UserID
		out.Body.Email = "user@example.com"
		out.Body.CreatedAt = "2024-01-15T10:00:00Z"
		return out, nil
	})
}

// --- Arrays ---

type TagsOutput struct {
	Body []string `json:"body" minItems:"1" maxItems:"10"`
}

func registerArrays(api huma.API) {
	huma.Get(api, "/tags", func(ctx context.Context, input *struct{}) (*TagsOutput, error) {
		return &TagsOutput{Body: []string{"a", "b", "c"}}, nil
	})
}

// --- Objects (additionalProperties) ---

type ConfigOutput struct {
	Body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"body"`
}

type MetadataOutput struct {
	Body map[string]string `json:"body"`
}

func registerObjects(api huma.API) {
	huma.Get(api, "/config", func(ctx context.Context, input *struct{}) (*ConfigOutput, error) {
		return &ConfigOutput{
			Body: struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{Key: "theme", Value: "dark"},
		}, nil
	})

	huma.Get(api, "/metadata", func(ctx context.Context, input *struct{}) (*MetadataOutput, error) {
		return &MetadataOutput{Body: map[string]string{"x": "1", "y": "2"}}, nil
	})
}

// --- Params (query, header) ---

type SearchInput struct {
	Limit  int    `query:"limit" default:"10" minimum:"1" maximum:"100" doc:"Page size"`
	Offset int    `query:"offset" minimum:"0" doc:"Offset"`
	Status string `query:"status" enum:"active,inactive,pending" doc:"Filter by status"`
}

type SearchOutput struct {
	Body struct {
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
		Status string `json:"status"`
	} `json:"body"`
}

type EchoInput struct {
	RequestID string `header:"X-Request-ID" doc:"Request ID"`
}

type EchoOutput struct {
	Body struct {
		RequestID string `json:"requestId"`
	} `json:"body"`
}

func registerParams(api huma.API) {
	huma.Get(api, "/search", func(ctx context.Context, input *SearchInput) (*SearchOutput, error) {
		return &SearchOutput{
			Body: struct {
				Limit  int    `json:"limit"`
				Offset int    `json:"offset"`
				Status string `json:"status"`
			}{Limit: input.Limit, Offset: input.Offset, Status: input.Status},
		}, nil
	})

	huma.Get(api, "/echo", func(ctx context.Context, input *EchoInput) (*EchoOutput, error) {
		return &EchoOutput{Body: struct {
			RequestID string `json:"requestId"`
		}{RequestID: input.RequestID}}, nil
	})
}

// --- Misc (deprecated, nullable, defaults) ---

type LegacyOutput struct {
	Body struct {
		Message string `json:"message"`
	} `json:"body"`
}

type NullableOutput struct {
	Body struct {
		Value *string `json:"value,omitempty" nullable:"true"`
		Num   *int    `json:"num,omitempty" nullable:"true"`
	} `json:"body"`
}

type DefaultsOutput struct {
	Body struct {
		Name string `json:"name" default:"unknown"`
		Size int    `json:"size" default:"0"`
	} `json:"body"`
}

func registerMisc(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getLegacy",
		Method:      http.MethodGet,
		Path:        "/legacy",
		Summary:     "Deprecated endpoint",
		Deprecated:  true,
	}, func(ctx context.Context, input *struct{}) (*LegacyOutput, error) {
		return &LegacyOutput{Body: struct {
			Message string `json:"message"`
		}{Message: "deprecated"}}, nil
	})

	huma.Get(api, "/nullable", func(ctx context.Context, input *struct{}) (*NullableOutput, error) {
		return &NullableOutput{
			Body: struct {
				Value *string `json:"value,omitempty" nullable:"true"`
				Num   *int    `json:"num,omitempty" nullable:"true"`
			}{Value: nil, Num: nil},
		}, nil
	})

	huma.Get(api, "/defaults", func(ctx context.Context, input *struct{}) (*DefaultsOutput, error) {
		return &DefaultsOutput{
			Body: struct {
				Name string `json:"name" default:"unknown"`
				Size int    `json:"size" default:"0"`
			}{Name: "foo", Size: 42},
		}, nil
	})
}

// --- Compositions (custom response bodies; schemas are what Huma generates from types, no patch for now) ---

type AllOfPart1 struct {
	A string `json:"a"`
}

type AllOfPart2 struct {
	B int `json:"b"`
}

type AllOfOutput struct {
	Body struct {
		AllOfPart1
		AllOfPart2
	} `json:"body"`
}

type AnyOfOutput struct {
	Body struct {
		Value string `json:"value"`
	} `json:"body"`
}

type OneOfOutput struct {
	Body struct {
		Type string `json:"type"`
		ID   int    `json:"id"`
	} `json:"body"`
}

func registerCompositions(api huma.API) {
	huma.Get(api, "/compositions/allof", func(ctx context.Context, input *struct{}) (*AllOfOutput, error) {
		return &AllOfOutput{
			Body: struct {
				AllOfPart1
				AllOfPart2
			}{AllOfPart1: AllOfPart1{A: "x"}, AllOfPart2: AllOfPart2{B: 2}},
		}, nil
	})

	huma.Get(api, "/compositions/anyof", func(ctx context.Context, input *struct{}) (*AnyOfOutput, error) {
		return &AnyOfOutput{Body: struct {
			Value string `json:"value"`
		}{Value: "string"}}, nil
	})

	huma.Get(api, "/compositions/oneof", func(ctx context.Context, input *struct{}) (*OneOfOutput, error) {
		return &OneOfOutput{Body: struct {
			Type string `json:"type"`
			ID   int    `json:"id"`
		}{Type: "first", ID: 1}}, nil
	})
}

// --- Discriminator (oneOf with kind) ---

type CircleShape struct {
	Kind   string  `json:"kind" enum:"circle"`
	Radius float64 `json:"radius"`
}

type RectangleShape struct {
	Kind   string  `json:"kind" enum:"rectangle"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ShapeOutput struct {
	Body json.RawMessage `json:"body"`
}

func registerDiscriminator(api huma.API) {
	huma.Get(api, "/shapes", func(ctx context.Context, input *struct{}) (*ShapeOutput, error) {
		body, _ := json.Marshal(map[string]any{
			"kind":   "circle",
			"radius": 5.0,
		})
		return &ShapeOutput{Body: body}, nil
	})
}

// --- Recursive (tree) ---

type TreeNode struct {
	Value    string     `json:"value"`
	Children []TreeNode `json:"children,omitempty"`
}

type TreeOutput struct {
	Body TreeNode `json:"body"`
}

func registerRecursive(api huma.API) {
	huma.Get(api, "/tree", func(ctx context.Context, input *struct{}) (*TreeOutput, error) {
		return &TreeOutput{
			Body: TreeNode{
				Value: "root",
				Children: []TreeNode{
					{Value: "a", Children: nil},
					{Value: "b", Children: []TreeNode{{Value: "b1", Children: nil}}},
				},
			},
		}, nil
	})
}
