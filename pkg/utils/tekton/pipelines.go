package tekton

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type PipelineRunGenerator interface {
	Generate() *v1beta1.PipelineRun
}

type BuildahDemo struct {
	Image  string
	Bundle string
}

// This is a demo pipeline to create test image and task signing
func (g BuildahDemo) Generate() *v1beta1.PipelineRun {
	imageInfo := strings.Split(g.Image, "/")
	namespace := imageInfo[1]
	// Make the PipelineRun name predictable.
	name := imageInfo[2]

	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			Params: []v1beta1.Param{
				{
					Name:  "dockerfile",
					Value: *v1beta1.NewArrayOrString("Dockerfile"),
				},
				{
					Name:  "output-image",
					Value: *v1beta1.NewArrayOrString(g.Image),
				},
				{
					Name:  "git-url",
					Value: *v1beta1.NewArrayOrString("https://github.com/ziwoshixianzhe/simple_docker_app.git"),
				},
			},
			PipelineRef: &v1beta1.PipelineRef{
				Name:   "docker-build",
				Bundle: g.Bundle,
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "app-studio-default-workspace",
					},
				},
			},
		},
	}
}

type VerifyEnterpriseContract struct {
	Bundle              string
	Image               string
	Name                string
	Namespace           string
	PolicyConfiguration string
	PublicKey           string
	SSLCertDir          string
	Strict              bool
}

func (p VerifyEnterpriseContract) Generate() *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-run-", p.Name),
			Namespace:    p.Namespace,
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name: "verify-enterprise-contract",
						Params: []v1beta1.Param{
							{
								Name: "IMAGES",
								Value: v1beta1.ArrayOrString{
									Type: v1beta1.ParamTypeString,
									StringVal: `{
							"application": "test",
							"components": [
							  {
								"name": "component1",
								"containerImage": "` + p.Image + `"
							  }
							]
						  }`,
								},
							},
							{
								Name: "POLICY_CONFIGURATION",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.PolicyConfiguration,
								},
							},
							{
								Name: "PUBLIC_KEY",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.PublicKey,
								},
							},
							{
								Name: "SSL_CERT_DIR",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: p.SSLCertDir,
								},
							},
							{
								Name: "STRICT",
								Value: v1beta1.ArrayOrString{
									Type:      v1beta1.ParamTypeString,
									StringVal: strconv.FormatBool(p.Strict),
								},
							},
						},
						TaskRef: &v1beta1.TaskRef{
							Name:   "verify-enterprise-contract",
							Bundle: p.Bundle,
						},
					},
				},
			},
		},
	}
}

func StreamRemoteYamlToTektonObj(url string, obj runtime.Object) (runtime.Object, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decodeBytesToTektonObjbytes(bytes, obj)
}

func decodeBytesToTektonObjbytes(bytes []byte, obj runtime.Object) (runtime.Object, error) {
	decodingScheme := runtime.NewScheme()
	utilruntime.Must(v1beta1.AddToScheme(decodingScheme))
	decoderCodecFactory := serializer.NewCodecFactory(decodingScheme)
	decoder := decoderCodecFactory.UniversalDecoder(v1beta1.SchemeGroupVersion)
	err := runtime.DecodeInto(decoder, bytes, obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
