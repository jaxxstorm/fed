// ui-contract derives browser component schemas from Go response types.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/jaxxstorm/fed/internal/assessment"
	"github.com/jaxxstorm/fed/internal/connections"
	"github.com/jaxxstorm/fed/internal/web"
	"os"
	"reflect"
	"strings"
	"time"
)

type schema map[string]any

var definitions = map[string]schema{}

func describe(t reflect.Type) schema {
	if t == reflect.TypeFor[time.Time]() {
		return schema{"type": "string", "format": "date-time"}
	}
	if t == reflect.TypeFor[json.RawMessage]() {
		return schema{"type": "object", "additionalProperties": true}
	}
	if t.Kind() == reflect.Pointer {
		return schema{"allOf": []schema{describe(t.Elem())}, "nullable": true}
	}
	switch t.Kind() {
	case reflect.String:
		return schema{"type": "string"}
	case reflect.Bool:
		return schema{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return schema{"type": "integer"}
	case reflect.Slice:
		return schema{"type": "array", "items": describe(t.Elem()), "nullable": true}
	case reflect.Map:
		return schema{"type": "object", "additionalProperties": describe(t.Elem()), "nullable": true}
	case reflect.Struct:
		name := t.Name()
		if _, exists := definitions[name]; exists {
			return schema{"$ref": "#/components/schemas/" + name}
		}
		definitions[name] = schema{}
		properties := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			if name == "-" {
				continue
			}
			if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
				describe(f.Type)
				embedded := definitions[f.Type.Name()]
				for key, value := range embedded["properties"].(map[string]any) {
					properties[key] = value
				}
				required = append(required, embedded["required"].([]string)...)
				continue
			}
			if name == "" {
				name = f.Name
			}
			properties[name] = describe(f.Type)
			if !strings.Contains(f.Tag.Get("json"), "omitempty") {
				required = append(required, name)
			}
		}
		definitions[name] = schema{"type": "object", "properties": properties, "required": required}
		return schema{"$ref": "#/components/schemas/" + name}
	default:
		return schema{}
	}
}
func main() {
	paths := map[string]any{}
	for path, typ := range map[string]reflect.Type{"/api/ui/v1/bootstrap": reflect.TypeFor[web.UIBootstrap](), "/api/ui/v1/services/{service}/overview": reflect.TypeFor[web.UIOverview](), "/api/ui/v1/services/{service}/pages/{page}": reflect.TypeFor[web.Page]()} {
		parameters := []any{}
		for _, name := range []string{"service", "page"} {
			if strings.Contains(path, "{"+name+"}") {
				parameters = append(parameters, schema{"name": name, "in": "path", "required": true, "schema": schema{"type": "string"}})
			}
		}
		paths[path] = schema{"get": schema{"parameters": parameters, "responses": schema{"200": schema{"description": "Authorized browser response", "content": schema{"application/json": schema{"schema": describe(typ)}}}}}}
	}
	describe(reflect.TypeFor[assessment.Record]())
	describe(reflect.TypeFor[web.UIError]())
	response := func(body schema) schema {
		return schema{"description": "Authorized browser response", "content": schema{"application/json": schema{"schema": body}}}
	}
	serviceParameter := schema{"name": "service", "in": "path", "required": true, "schema": schema{"type": "string"}}
	recordSchema := schema{"$ref": "#/components/schemas/Record"}
	records := schema{"type": "object", "required": []string{"items", "has_more", "offset"}, "properties": schema{"items": schema{"type": "array", "items": recordSchema}, "has_more": schema{"type": "boolean"}, "offset": schema{"type": "integer"}}}
	parameters := []any{serviceParameter, schema{"name": "kind", "in": "path", "required": true, "schema": schema{"type": "string", "enum": []string{"evidence", "rule_result", "evidence_source", "snapshot", "publication"}}}}
	for _, name := range []string{"offset", "q", "source", "freshness", "requirement", "type"} {
		parameters = append(parameters, schema{"name": name, "in": "query", "schema": schema{"type": "string"}})
	}
	paths["/api/ui/v1/services/{service}/records/{kind}"] = schema{"get": schema{"parameters": parameters, "responses": schema{"200": response(records)}}}
	paths["/api/ui/v1/services/{service}/pages/{page}/{requirement}"] = schema{"get": schema{"parameters": []any{serviceParameter, schema{"name": "page", "in": "path", "required": true, "schema": schema{"type": "string", "enum": []string{"requirement", "run"}}}, schema{"name": "requirement", "in": "path", "required": true, "schema": schema{"type": "string"}}}, "responses": schema{"200": response(describe(reflect.TypeFor[web.Page]()))}}}
	paths["/api/ui/v1/services/{service}/preview/{id}"] = schema{"get": schema{"parameters": []any{serviceParameter, schema{"name": "id", "in": "path", "required": true, "schema": schema{"type": "string"}}, schema{"name": "revision", "in": "query", "required": true, "schema": schema{"type": "integer", "minimum": 1}}}, "responses": schema{"200": response(schema{"type": "object", "required": []string{"content"}, "properties": schema{"content": schema{"type": "string"}}})}}}
	for _, path := range []string{"/api/ui/v1/services", "/api/ui/v1/services/{service}/actions/{action}"} {
		parameters := []any{schema{"name": "X-CSRF-Token", "in": "header", "required": true, "schema": schema{"type": "string"}}}
		if strings.Contains(path, "{service}") {
			parameters = append(parameters, serviceParameter, schema{"name": "action", "in": "path", "required": true, "schema": schema{"type": "string", "enum": []string{"evidence-source", "collect", "collection-config", "scope", "preview-profile", "profile", "measure", "evidence", "draft", "review", "gap", "check", "token", "revoke-token", "destination", "prepare"}}})
		}
		paths[path] = schema{"post": schema{"parameters": parameters, "requestBody": schema{"required": true, "content": schema{"application/x-www-form-urlencoded": schema{"schema": schema{"type": "object", "additionalProperties": schema{"type": "string"}}}}}, "responses": schema{"200": response(schema{"oneOf": []schema{describe(reflect.TypeFor[web.Page]()), {"type": "object", "required": []string{"redirect"}, "properties": schema{"redirect": schema{"type": "string"}}}}}), "400": response(describe(reflect.TypeFor[web.UIError]())), "403": response(describe(reflect.TypeFor[web.UIError]())), "409": response(describe(reflect.TypeFor[web.UIError]()))}}}
	}
	out := schema{"openapi": "3.0.3", "info": schema{"title": "Assessment browser API", "version": "1"}, "paths": paths, "components": schema{"schemas": definitions}}
	connectionBase := "/api/ui/v1/services/{service}/connections"
	paths[connectionBase+"/scope"] = schema{"get": schema{"parameters": []any{serviceParameter}, "responses": schema{"200": response(describe(reflect.TypeFor[assessment.Record]()))}}}
	connectionParameter := schema{"name": "connection", "in": "path", "required": true, "schema": schema{"type": "string"}}
	for suffix, typ := range map[string]reflect.Type{"": reflect.TypeFor[[]connections.Detail](), "/providers": reflect.TypeFor[[]connections.ProviderDefinition](), "/{connection}": reflect.TypeFor[connections.Detail](), "/{connection}/inventory": reflect.TypeFor[connections.InventoryPage](), "/operations/{operation}/status": reflect.TypeFor[connections.Operation]()} {
		parameters := []any{serviceParameter}
		if strings.Contains(suffix, "{connection}") {
			parameters = append(parameters, connectionParameter)
		}
		if strings.Contains(suffix, "{operation}") {
			parameters = append(parameters, schema{"name": "operation", "in": "path", "required": true, "schema": schema{"type": "string"}})
		}
		paths[connectionBase+suffix] = schema{"get": schema{"parameters": parameters, "responses": schema{"200": response(describe(typ))}}}
	}
	type actionContract struct{ Input, Output reflect.Type }
	actions := map[string]actionContract{
		"resolve-commit":   {reflect.TypeFor[connections.ResolveCommitInput](), reflect.TypeFor[connections.ResolvedCommit]()},
		"configure":        {reflect.TypeFor[connections.ConfigureInput](), reflect.TypeFor[connections.Detail]()},
		"test":             {reflect.TypeFor[connections.OperationInput](), reflect.TypeFor[connections.Operation]()},
		"renew":            {reflect.TypeFor[connections.OperationInput](), reflect.TypeFor[connections.Operation]()},
		"authorize":        {reflect.TypeFor[connections.OperationInput](), reflect.TypeFor[connections.AuthorizationHandoff]()},
		"discover":         {reflect.TypeFor[connections.OperationInput](), reflect.TypeFor[connections.DiscoverySession]()},
		"discover-page":    {reflect.TypeFor[connections.ServerDiscoveryInput](), reflect.TypeFor[connections.DiscoveryReceipt]()},
		"select":           {reflect.TypeFor[connections.SelectionInput](), reflect.TypeFor[connections.SelectionReceipt]()},
		"configure-source": {reflect.TypeFor[connections.SourceConfigurationInput](), reflect.TypeFor[assessment.Record]()},
		"refresh-source":   {reflect.TypeFor[connections.RefreshSourceInput](), reflect.TypeFor[assessment.Record]()},
	}
	for _, name := range []string{"disconnect", "delete", "restore"} {
		actions[name] = actionContract{reflect.TypeFor[connections.OperationInput](), reflect.TypeFor[connections.Detail]()}
	}
	for _, name := range []string{"retire", "disable", "restore", "rebind"} {
		actions[name+"-source"] = actionContract{reflect.TypeFor[connections.SourceOperationInput](), reflect.TypeFor[assessment.Record]()}
	}
	for name, contract := range actions {
		paths[connectionBase+"/"+name] = schema{"post": schema{"parameters": []any{serviceParameter, schema{"name": "X-CSRF-Token", "in": "header", "required": true, "schema": schema{"type": "string"}}}, "requestBody": schema{"required": true, "content": schema{"application/json": schema{"schema": describe(contract.Input)}}}, "responses": schema{"200": response(describe(contract.Output)), "400": response(describe(reflect.TypeFor[web.UIError]())), "403": response(describe(reflect.TypeFor[web.UIError]())), "409": response(describe(reflect.TypeFor[web.UIError]()))}}}
	}
	for _, provider := range []string{"kubernetes", "aws"} {
		field := "kubeconfig"
		if provider == "aws" {
			field = "external_id"
		}
		paths["/connections/upload/"+provider] = schema{"post": schema{"requestBody": schema{"required": true, "content": schema{"application/x-www-form-urlencoded": schema{"schema": schema{"type": "object", "required": []string{"csrf_token", "state", field}, "properties": schema{"csrf_token": schema{"type": "string", "writeOnly": true}, "state": schema{"type": "string", "writeOnly": true}, field: schema{"type": "string", "writeOnly": true, "format": "password"}}}}}}, "responses": schema{"303": schema{"description": "Redirect to the source connection and resumable operation status"}, "403": response(describe(reflect.TypeFor[web.UIError]()))}}}
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(os.Stdout, string(raw))
}
