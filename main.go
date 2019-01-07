package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/sprig"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
	"github.com/imdario/mergo"
	"path"
	"reflect"
	"text/template"
)

func hash(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])
}

func renderFile(d *schema.ResourceData) (string, error) {
	var err error
	// Get the data from terraform
	var data = make(map[string]interface{})
	for _, input := range d.Get("inputs").([]interface{}) {
		tmp := make(map[string]interface{})
		if err = json.Unmarshal([]byte(input.(string)), &tmp); err != nil {
			return "", fmt.Errorf("Failed to unmarshal string as json: %v", err)
		}
		if err = mergo.Merge(&data, tmp, mergo.WithAppendSlice); err != nil {
			return "", fmt.Errorf("Failed merging json snippets: %v", err)
		}
	}
	// Acquire the list of templates
	var templateFiles = make([]string, 0)
	for _, templateFile := range d.Get("templates").([]interface{}) {
		templateFiles = append(templateFiles, templateFile.(string))
	}
	baseName := path.Base(templateFiles[0]) // Use first templatefile as name
	t := template.New(baseName)
	// Create the function map
	funcMap := template.FuncMap{
		"isInt": func(i interface{}) bool {
			v := reflect.ValueOf(i)
			switch v.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
				return true
			default:
				return false
			}
		},
		"isString": func(i interface{}) bool {
			v := reflect.ValueOf(i)
			switch v.Kind() {
			case reflect.String:
				return true
			default:
				return false
			}
		},
		"isSlice": func(i interface{}) bool {
			v := reflect.ValueOf(i)
			switch v.Kind() {
			case reflect.Slice:
				return true
			default:
				return false
			}
		},
		"isArray": func(i interface{}) bool {
			v := reflect.ValueOf(i)
			switch v.Kind() {
			case reflect.Array:
				return true
			default:
				return false
			}
		},
		"isMap": func(i interface{}) bool {
			v := reflect.ValueOf(i)
			switch v.Kind() {
			case reflect.Map:
				return true
			default:
				return false
			}
		},
		// Helper functions similar to helm
		"include": func(name string, data interface{}) (string, error) {
			buf := bytes.NewBuffer(nil)
			if err := t.ExecuteTemplate(buf, name, data); err != nil {
				return "", err
			}
			return buf.String(), nil
		},
		"required": func(warn string, val interface{}) (interface{}, error) {
			if val == nil {
				// Convert nil to "" in case required is piped into other functions
				return "", fmt.Errorf(warn)
			} else if _, ok := val.(string); ok {
				if val == "" {
					return val, fmt.Errorf(warn)
				}
			}
			return val, nil
		},
	}
	// Add all sprig functions
	for k, v := range sprig.TxtFuncMap() {
		funcMap[k] = v
	}
	// Add the functions and parse the templates
	t, err = t.Funcs(funcMap).ParseFiles(templateFiles...)
	if err != nil {
		return "", fmt.Errorf("Failed parsing templates: %v", err)
	}
	// Execute the template
	var contents bytes.Buffer // io.writer for template.Execute
	if t != nil {
		err = t.Execute(&contents, data)
		if err != nil {
			return "", fmt.Errorf("Failed to execute template: %v", err)
		}
	} else {
		return "", fmt.Errorf("Unknown error: %v", err)
	}

	return contents.String(), nil
}

func dataSourceFileRead(d *schema.ResourceData, meta interface{}) error {
	rendered, err := renderFile(d)
	if err != nil {
		return err
	}
	d.Set("rendered", rendered)
	d.SetId(hash(rendered))
	return nil
}

func dataSourceFile() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceFileRead,

		Schema: map[string]*schema.Schema{
			"templates": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required:    true,
				Description: "path to go template file",
			},
			"inputs": &schema.Schema{
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Required:    true,
				Description: "variables to substitute",
			},
			"rendered": &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "rendered template",
			},
		},
	}
}

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() terraform.ResourceProvider {
			return &schema.Provider{
				DataSourcesMap: map[string]*schema.Resource{
					"gotemplate": dataSourceFile(),
				},
			}
		},
	})
}
