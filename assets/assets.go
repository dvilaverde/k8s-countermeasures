package assets

import (
	"bytes"
	"embed"
	"text/template"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	//go:embed manifests/*
	manifests embed.FS
	scheme    = runtime.NewScheme()
)

func init() {

	if err := batchv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
}

func GetPatch(name string) client.Patch {

	fn := make(template.FuncMap)
	fn["now"] = func() (string, error) {
		return time.Now().Format(time.RFC3339), nil
	}

	tmpl, err := template.New(name).Funcs(fn).ParseFS(manifests, "manifests/"+name)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, name, nil)
	if err != nil {
		panic(err)
	}

	json, err := yaml.YAMLToJSON(buf.Bytes())
	if err != nil {
		panic(err)
	}

	return client.RawPatch(types.MergePatchType, json)
}
