package assets

import (
	"embed"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	//go:embed manifests/*
	manifests embed.FS
	scheme    = runtime.NewScheme()
	codecs    = serializer.NewCodecFactory(scheme)
)

func init() {

	if err := batchv1.AddToScheme(scheme); err != nil {
		panic(err)
	}
}

func GetJobFromFile(name string) *batchv1.Job {
	jobBytes, err := manifests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	jobObject, err := runtime.Decode(codecs.UniversalDecoder(batchv1.SchemeGroupVersion), jobBytes)
	if err != nil {
		panic(err)
	}

	return jobObject.(*batchv1.Job)
}
