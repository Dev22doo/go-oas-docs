package docs

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultDocsOutPath = "./internal/dist/openapi.yaml"

// ConfigBuilder represents a config structure which will be used for the YAML Builder (BuildDocs fn).
//
// This structure was introduced to enable possible extensions to the OAS.BuildDocs() without introducing breaking API changes.
type ConfigBuilder struct {
	customPath string
}

func (cb ConfigBuilder) getPath() string {
	return cb.customPath
}

func getPathFromFirstElement(cbs []ConfigBuilder) string {
	if len(cbs) == 0 {
		return defaultDocsOutPath
	}

	return cbs[0].getPath()
}

// BuildDocs marshals the OAS struct to YAML and saves it to the chosen output file.
//
// Returns an error if there is any.
func (o *OAS) BuildDocs(conf ...ConfigBuilder) error {
	err := o.initCallStackForRoutes()
	if err != nil {
		return fmt.Errorf("failed initiating call stack for registered routes: %w", err)
	}

	yml, err := marshalToYAML(o)
	if err != nil {
		return fmt.Errorf("marshaling issue occurred: %w", err)
	}

	err = createYAMLOutFile(getPathFromFirstElement(conf), yml)
	if err != nil {
		return fmt.Errorf("an issue occurred while saving to YAML output: %w", err)
	}

	return nil
}

func marshalToYAML(oas *OAS) ([]byte, error) {
	transformedOAS := oas.transformToHybridOAS()

	yml, err := yaml.Marshal(transformedOAS)
	if err != nil {
		return yml, fmt.Errorf("failed marshaling to yaml: %w", err)
	}

	return yml, err
}

func createYAMLOutFile(outPath string, marshaledYAML []byte) error {
	outYAML, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed creating yaml output file: %w", err)
	}
	defer outYAML.Close()

	err = writeAndFlush(marshaledYAML, outYAML)
	if err != nil {
		return fmt.Errorf("writing issue occurred: %w", err)
	}

	return nil
}

func writeAndFlush(yml []byte, outYAML *os.File) error {
	writer := bufio.NewWriter(outYAML)

	_, err := writer.Write(yml)
	if err != nil {
		return fmt.Errorf("failed writing to YAML output file: %w", err)
	}

	err = writer.Flush()
	if err != nil {
		return fmt.Errorf("failed flushing output writer: %w", err)
	}

	return nil
}

const (
	keyTags            = "tags"
	keySummary         = "summary"
	keyOperationID     = "operationId"
	keySecurity        = "security"
	keyRequestBody     = "requestBody"
	keyResponses       = "responses"
	keyDescription     = "description"
	keyContent         = "content"
	keyRef             = "$ref"
	keySchemas         = "schemas"
	keySecuritySchemes = "securitySchemes"
	keyName            = "name"
	keyType            = "type"
	keyProperties      = "properties"
	keyIn              = "in"
	keyXML             = "xml"
)

// TODO: Should I add hash linked list maps support?
type (
	pathsMap         map[string]methodsMap
	componentsMap    map[string]interface{}
	methodsMap       map[string]interface{}
	pathSecurityMap  map[string][]string
	pathSecurityMaps []pathSecurityMap
)

type hybridOAS struct {
	OpenAPI      OASVersion    `yaml:"openapi"`
	Info         Info          `yaml:"info"`
	ExternalDocs ExternalDocs  `yaml:"externalDocs"`
	Servers      Servers       `yaml:"servers"`
	Tags         Tags          `yaml:"tags"`
	Paths        pathsMap      `yaml:"paths"`
	Components   componentsMap `yaml:"components"`
}

func (o *OAS) transformToHybridOAS() hybridOAS {
	ho := hybridOAS{}

	ho.OpenAPI = o.OASVersion
	ho.Info = o.Info
	ho.ExternalDocs = o.ExternalDocs
	ho.Servers = o.Servers
	ho.Tags = o.Tags

	ho.Paths = makeAllPathsMap(&o.Paths)
	ho.Components = makeComponentsMap(&o.Components)

	return ho
}

func makeAllPathsMap(paths *Paths) pathsMap {
	allPaths := make(pathsMap, len(*paths))
	for _, path := range *paths { //nolint:gocritic //consider indexing?
		if allPaths[path.Route] == nil {
			allPaths[path.Route] = make(methodsMap)
		}

		pathMap := make(map[string]interface{})
		pathMap[keyTags] = path.Tags
		pathMap[keySummary] = path.Summary
		pathMap[keyOperationID] = path.OperationID
		pathMap[keySecurity] = makeSecurityMap(&path.Security)
		pathMap[keyRequestBody] = makeRequestBodyMap(&path.RequestBody)
		pathMap[keyResponses] = makeResponsesMap(&path.Responses)

		allPaths[path.Route][strings.ToLower(path.HTTPMethod)] = pathMap
	}

	return allPaths
}

func makeRequestBodyMap(reqBody *RequestBody) map[string]interface{} {
	reqBodyMap := make(map[string]interface{})

	reqBodyMap[keyDescription] = reqBody.Description
	reqBodyMap[keyContent] = makeContentSchemaMap(reqBody.Content)

	return reqBodyMap
}

func makeResponsesMap(responses *Responses) map[uint]interface{} {
	responsesMap := make(map[uint]interface{}, len(*responses))

	for _, resp := range *responses {
		codeBodyMap := make(map[string]interface{})
		codeBodyMap[keyDescription] = resp.Description
		codeBodyMap[keyContent] = makeContentSchemaMap(resp.Content)

		responsesMap[resp.Code] = codeBodyMap
	}

	return responsesMap
}

func makeSecurityMap(se *SecurityEntities) pathSecurityMaps {
	securityMaps := make(pathSecurityMaps, 0, len(*se))

	for _, sec := range *se {
		securityMap := make(pathSecurityMap)
		securityMap[sec.AuthName] = sec.PermTypes

		securityMaps = append(securityMaps, securityMap)
	}

	return securityMaps
}

func makeContentSchemaMap(content ContentTypes) map[string]interface{} {
	contentSchemaMap := make(map[string]interface{})

	for _, ct := range content {
		refMap := make(map[string]string)
		refMap[keyRef] = ct.Schema

		schemaMap := make(map[string]map[string]string)
		schemaMap["schema"] = refMap

		contentSchemaMap[ct.Name] = schemaMap
	}

	return contentSchemaMap
}

func makeComponentsMap(components *Components) componentsMap {
	cm := make(componentsMap, len(*components))

	for _, component := range *components {
		cm[keySchemas] = makeComponentSchemasMap(&component.Schemas)
		cm[keySecuritySchemes] = makeComponentSecuritySchemesMap(&component.SecuritySchemes)
	}

	return cm
}

func makeComponentSchemasMap(schemas *Schemas) map[string]interface{} {
	schemesMap := make(map[string]interface{}, len(*schemas))

	for _, s := range *schemas {
		scheme := make(map[string]interface{})
		scheme[keyType] = s.Type
		scheme[keyProperties] = s.Properties
		scheme[keyRef] = s.Ref

		if s.XML.Name != "" {
			scheme[keyXML] = s.XML
		}

		schemesMap[s.Name] = scheme
	}

	return schemesMap
}

func makeComponentSecuritySchemesMap(secSchemes *SecuritySchemes) map[string]interface{} {
	secSchemesMap := make(map[string]interface{}, len(*secSchemes))

	for _, ss := range *secSchemes {
		scheme := make(map[string]interface{})
		scheme[keyName] = ss.Name
		scheme[keyType] = ss.Type

		if ss.In != "" {
			scheme[keyIn] = ss.In
		}

		secSchemesMap[ss.Name] = scheme
	}

	return secSchemesMap
}
