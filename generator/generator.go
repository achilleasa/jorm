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

	var renderers []templateRenderer
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
var auxFuncs = template.FuncMap{}
