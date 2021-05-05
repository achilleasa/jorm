package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	goparser "go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/achilleasa/jorm/generator/parser"
)

var (
	schemaPkg = flag.String("schema-pkg", "github.com/achilleasa/jorm/schema", "the package containing the model schema definitions")
	modelPkg  = flag.String("model-pkg", "github.com/achilleasa/jorm/model", "the package where the generated models will be stored")
	storePkg  = flag.String("store-pkg", "github.com/achilleasa/jorm/store", "the package where the generated store interfaces be stored")
)

func main() {
	if err := runGenerator(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

type templateRenderer struct {
	templateFile   string
	targetPackage  string
	targetFileFunc func(*parser.Model) string
	runPerSrcFile  bool
}

func runGenerator() error {
	flag.Parse()

	models, err := parser.ParseModelSchemas(*schemaPkg, *modelPkg)
	if err != nil {
		return fmt.Errorf("unable to parse model schemas: %w", err)
	}

	// The model generator template uses a uint64 bitmap (bits 2-63) to
	// track per-field mutations. As such, there is a max limit of 61 fields
	// per model.
	for _, model := range models.All() {
		if got := len(model.Fields); got > 61 {
			return fmt.Errorf("model %s defines %d fields whereas the ORM implementation details impose a limit of 61 fields per model", model.Name.Public, got)
		}
	}

	var renderers = []templateRenderer{
		{
			templateFile:  "tpl/model_gen.go.tpl",
			targetPackage: *modelPkg,
			targetFileFunc: func(model *parser.Model) string {
				return strings.Replace(
					filepath.Base(model.SrcFile),
					".go",
					"_gen.go",
					1,
				)
			},
			runPerSrcFile: true,
		},
		{
			templateFile:  "tpl/accessor_gen.go.tpl",
			targetPackage: *storePkg,
			targetFileFunc: func(model *parser.Model) string {
				return strings.Replace(
					filepath.Base(model.SrcFile),
					".go",
					"_accessor_gen.go",
					1,
				)
			},
			runPerSrcFile: true,
		},
	}
	return renderTemplates(models, renderers)
}

type templateData struct {
	// The package where the rendered files are to be placed.
	TargetPkg string

	// The list of models passed to the template.
	Models []*parser.Model

	// The package where the model files are to be placed.
	ModelPkg string
}

func renderTemplates(models *parser.Models, renderers []templateRenderer) error {
	// Load and compile templates first.
	tpls := make([]*template.Template, len(renderers))
	for i, renderer := range renderers {
		tplData, err := ioutil.ReadFile(renderer.templateFile)
		if err != nil {
			return fmt.Errorf("error while reading model template: %w", err)
		}

		if tpls[i], err = template.New(renderer.templateFile).Funcs(auxFuncs).Parse(string(tplData)); err != nil {
			return fmt.Errorf("error while compiling model template %q: %w", renderer.templateFile, err)
		}
	}

	// Run each renderer and store buffer their output. The output files
	// will only be written to disk if all renderer steps complete without
	// an error.
	var outputByFile = make(map[string]string)

	for rIdx, renderer := range renderers {
		if renderer.runPerSrcFile {
			for srcFile, modelDefsInFile := range models.ModelsBySrcFile() {
				targetFile := filepath.Join(os.Getenv("GOPATH"), "src", renderer.targetPackage, renderer.targetFileFunc(modelDefsInFile[0]))
				output, err := renderTemplate(targetFile, tpls[rIdx], templateData{
					TargetPkg: renderer.targetPackage,
					Models:    modelDefsInFile,
					ModelPkg:  *modelPkg,
				})
				if err != nil {
					return fmt.Errorf("error while rendering models from file %q: %w", srcFile, err)
				}
				outputByFile[targetFile] = output
			}
		} else {
			targetFile := filepath.Join(os.Getenv("GOPATH"), "src", renderer.targetPackage, renderer.targetFileFunc(nil))
			output, err := renderTemplate(targetFile, tpls[rIdx], templateData{
				TargetPkg: renderer.targetPackage,
				Models:    models.All(),
				ModelPkg:  *modelPkg,
			})
			if err != nil {
				return fmt.Errorf("error while rendering models: %w", err)
			}
			outputByFile[targetFile] = output
		}
	}

	// As no error occurred, we can now write the output to disk.
	for targetFile, content := range outputByFile {
		fmt.Fprintf(os.Stderr, "writing generated file: %s\n", targetFile)
		path := filepath.Dir(targetFile)
		if err := os.MkdirAll(path, 0777); err != nil {
			return err
		}
		if err := ioutil.WriteFile(targetFile, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func renderTemplate(targetFile string, tpl *template.Template, data templateData) (string, error) {
	var buf bytes.Buffer

	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error while rendering template: %w", err)
	}

	// Ensure that rendered model can be parsed.
	fset := token.NewFileSet()
	_, err := goparser.ParseFile(fset, targetFile, buf.Bytes(), goparser.ParseComments)
	if err != nil {
		lines := strings.Split(buf.String(), "\n")
		for i := 0; i < len(lines); i++ {
			lines[i] = fmt.Sprintf("%04d | %s", i, lines[i])
		}
		return "", fmt.Errorf("error while parsing rendered model output: %w\n\nOutput was:\n%s", err, strings.Join(lines, "\n"))
	}

	// Pass output through gofmt.
	fmtOutput, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("error while pretty-printing rendered output: %w", err)
	}

	return string(fmtOutput), nil
}

// The list of aux functions that are available to all templates.
var auxFuncs = template.FuncMap{
	// Return the base name for a package.
	"basePkgName": func(pkg string) string {
		return filepath.Base(pkg)
	},
	// Remove all selector ("$pkg.") instances from the specified type.
	"trimPkgSelector": func(pkg, goType string) string {
		return strings.Replace(
			goType,
			fmt.Sprintf("%s.", filepath.Base(pkg)),
			"",
			-1,
		)
	},
	// Return the unique set of imports for referencing models/fields
	// for the specified models.
	"requiredPkgImports": func(models []*parser.Model) []string {
		set := make(map[string]struct{})
		for _, model := range models {
			for _, field := range model.Fields {
				for _, pkgName := range field.Type.RequiredImports {
					set[pkgName] = struct{}{}
				}
			}
		}

		var uniqueSet []string
		for pkgName := range set {
			uniqueSet = append(uniqueSet, pkgName)
		}
		return uniqueSet
	},
	// Filter input model list and return the non-embedded models.
	"nonEmbeddedModels": func(models []*parser.Model) []*parser.Model {
		var filteredModels []*parser.Model
		for _, model := range models {
			if !model.IsEmbdedded {
				filteredModels = append(filteredModels, model)
			}
		}
		return filteredModels
	},
	// Filter input model list and return the embedded models.
	"embeddedModels": func(models []*parser.Model) []*parser.Model {
		var filteredModels []*parser.Model
		for _, model := range models {
			if model.IsEmbdedded {
				filteredModels = append(filteredModels, model)
			}
		}
		return filteredModels
	},
	// Return the correct getter name (GetX or IsX) for a field.
	"getter": func(field *parser.Field) string {
		if field.Type.Resolved == "*bool" || field.Type.Resolved == "bool" {
			if strings.HasPrefix(field.Name.Public, "Has") {
				return field.Name.Public
			}
			return fmt.Sprintf("Is%s", field.Name.Public)
		}
		return fmt.Sprintf("Get%s", field.Name.Public)
	},
}
