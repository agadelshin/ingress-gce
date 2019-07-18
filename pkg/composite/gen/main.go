/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Generator for GCE compute wrapper code. You must regenerate the code after
// modifying this file:
//
//   $ ./hack/update_codegen.sh

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	compositemeta "k8s.io/ingress-gce/pkg/composite/meta"
)

const (
	gofmt = "gofmt"
)

// gofmtContent runs "gofmt" on the given contents.
func gofmtContent(r io.Reader) string {
	cmd := exec.Command(gofmt, "-s")
	out := &bytes.Buffer{}
	cmd.Stdin = r
	cmd.Stdout = out
	cmdErr := &bytes.Buffer{}
	cmd.Stderr = cmdErr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, cmdErr.String())
		panic(err)
	}
	return out.String()
}

func genHeader(wr io.Writer) {
	const text = `/*
Copyright {{.Year}} The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// This file was generated by "./hack/update-codegen.sh". Do not edit directly.
// directly.

package composite
import (
	"fmt"

	"k8s.io/klog"
	computealpha "google.golang.org/api/compute/v0.alpha"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter"
	cloudprovider "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	compositemetrics "k8s.io/ingress-gce/pkg/composite/metrics"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)
`
	tmpl := template.Must(template.New("header").Parse(text))
	values := map[string]string{
		"Year": fmt.Sprintf("%v", time.Now().Year()),
	}
	if err := tmpl.Execute(wr, values); err != nil {
		panic(err)
	}

	fmt.Fprintf(wr, "\n\n")
}

func genTestHeader(wr io.Writer) {
	const text = `/*
Copyright {{.Year}} The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// This file was generated by "./hack/update-codegen.sh". Do not edit directly.
// directly.

package composite
import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kr/pretty"
	computealpha "google.golang.org/api/compute/v0.alpha"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
)
`
	tmpl := template.Must(template.New("testHeader").Parse(text))
	values := map[string]string{
		"Year": fmt.Sprintf("%v", time.Now().Year()),
	}
	if err := tmpl.Execute(wr, values); err != nil {
		panic(err)
	}

	fmt.Fprintf(wr, "\n\n")
}

// genTypes() generates all of the composite structs.
func genTypes(wr io.Writer) {
	const text = `
{{ $backtick := "` + "`" + `" }}
{{- range .All}}
	// {{.Name}} is a composite type wrapping the Alpha, Beta, and GA methods for its GCE equivalent
	type {{.Name}} struct {
		{{- if .IsMainService}}
			// Version keeps track of the intended compute version for this {{.Name}}.
			// Note that the compute API's do not contain this field. It is for our
			// own bookkeeping purposes.
			Version meta.Version {{$backtick}}json:"-"{{$backtick}}
			// Scope keeps track of the intended type of the service (e.g. Global)
			// This is also an internal field purely for bookkeeping purposes
			Scope meta.KeyType {{$backtick}}json:"-"{{$backtick}}
		{{end}}

		{{- range .Fields}}
			{{.Description}}
			{{- if eq .Name "Id"}}
				{{- .Name}} {{.GoType}} {{$backtick}}json:"{{.JsonName}},omitempty,string"{{$backtick}}
			{{- else if .JsonStringOverride}}
				{{- .Name}} {{.GoType}} {{$backtick}}json:"{{.JsonName}},omitempty,string"{{$backtick}}
			{{- else}}
				{{- .Name}} {{.GoType}} {{$backtick}}json:"{{.JsonName}},omitempty"{{$backtick}}
			{{- end}}
		{{- end}}
		{{- if .IsMainService}}
			googleapi.ServerResponse {{$backtick}}json:"-"{{$backtick}}
		{{- end}}
		ForceSendFields []string {{$backtick}}json:"-"{{$backtick}}
		NullFields []string {{$backtick}}json:"-"{{$backtick}}
}
{{- end}}
`
	data := struct {
		All []compositemeta.ApiService
	}{compositemeta.AllApiServices}

	tmpl := template.Must(template.New("types").Parse(text))
	if err := tmpl.Execute(wr, data); err != nil {
		panic(err)
	}
}

// genFuncs() generates helper methods attached to composite structs.
// TODO: (shance) Fix force send fields hack
// TODO: (shance) Have To*() Functions set ResourceType and Version fields
// TODO: (shance) Figure out a better solution so that the List() functions don't have to take a meta.Key
// that ignores the name field
func genFuncs(wr io.Writer) {
	const text = `
{{$All := .All}}
{{$Versions := .Versions}}

{{range $type := $All}}
{{if .IsMainService}}
	{{if .IsDefaultRegionalService}}
  func Create{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, {{.VarName}} *{{.Name}}) error {
		ctx, cancel := cloudprovider.ContextWithCallTimeout()
		defer cancel()
  	mc := compositemetrics.NewMetricContext("{{.Name}}", "create", key.Region, key.Zone, string({{.VarName}}.Version))

		switch {{.VarName}}.Version {
		case meta.VersionAlpha:
			alpha, err := {{.VarName}}.ToAlpha()
			if err != nil {
				return err
			}
			switch key.Type() {
			case meta.Regional:
				klog.V(3).Infof("Creating alpha region {{.Name}} %v", alpha.Name)
      	alpha.Region = key.Region
				return mc.Observe(gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Insert(ctx, key, alpha))
			default:
				klog.V(3).Infof("Creating alpha {{.Name}} %v", alpha.Name)
				return mc.Observe(gceCloud.Compute().AlphaGlobal{{.GetCloudProviderName}}().Insert(ctx, key, alpha))
		}
		case meta.VersionBeta:
			beta, err := {{.VarName}}.ToBeta()
			if err != nil {
				return err
			}
			switch key.Type() {
			case meta.Regional:
		  	klog.V(3).Infof("Creating beta region {{.Name}} %v", beta.Name)
				return mc.Observe(gceCloud.Compute().Beta{{.GetCloudProviderName}}().Insert(ctx, key, beta))
			default:
				klog.V(3).Infof("Creating beta {{.Name}} %v", beta.Name)
				return mc.Observe(gceCloud.Compute().BetaGlobal{{.GetCloudProviderName}}().Insert(ctx, key, beta))
	}
		default:
			ga, err := {{.VarName}}.ToGA()
			if err != nil {
				return err
			}
			switch key.Type() {
			case meta.Regional:
		  	klog.V(3).Infof("Creating ga region {{.Name}} %v", ga.Name)
				return mc.Observe(gceCloud.Compute().{{.GetCloudProviderName}}().Insert(ctx, key, ga))
			default:
		  	klog.V(3).Infof("Creating ga {{.Name}} %v", ga.Name)
				return mc.Observe(gceCloud.Compute().Global{{.GetCloudProviderName}}().Insert(ctx, key, ga))
		}}
}

{{if .HasUpdate}}
func Update{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, {{.VarName}} *{{.Name}}) error {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "update", key.Region, key.Zone, string({{.VarName}}.Version))

	switch {{.VarName}}.Version {
	case meta.VersionAlpha:
		alpha, err := {{.VarName}}.ToAlpha()
		if err != nil {
			return err
		}
		switch key.Type() {
		case meta.Regional:
		  klog.V(3).Infof("Updating alpha region {{.Name}} %v", alpha.Name)
			return mc.Observe(gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Update(ctx, key, alpha))
		default:
		  klog.V(3).Infof("Updating alpha {{.Name}} %v", alpha.Name)
			return mc.Observe(gceCloud.Compute().AlphaGlobal{{.GetCloudProviderName}}().Update(ctx, key, alpha))
		}
	case meta.VersionBeta:
		beta, err := {{.VarName}}.ToBeta()
		if err != nil {
			return err
		}
		switch key.Type() {
		case meta.Regional:
  		klog.V(3).Infof("Updating beta region {{.Name}} %v", beta.Name)
			return mc.Observe(gceCloud.Compute().Beta{{.GetCloudProviderName}}().Update(ctx, key, beta))
		default:
  		klog.V(3).Infof("Updating beta {{.Name}} %v", beta.Name)
			return mc.Observe(gceCloud.Compute().BetaGlobal{{.GetCloudProviderName}}().Update(ctx, key, beta))
		}	default:
		ga, err := {{.VarName}}.ToGA()
		if err != nil {
			return err
		}
		switch key.Type() {
		case meta.Regional:
  		klog.V(3).Infof("Updating ga region {{.Name}} %v", ga.Name)
			return mc.Observe(gceCloud.Compute().{{.GetCloudProviderName}}().Update(ctx, key, ga))
		default:
  		klog.V(3).Infof("Updating ga {{.Name}} %v", ga.Name)
			return mc.Observe(gceCloud.Compute().Global{{.GetCloudProviderName}}().Update(ctx, key, ga))
		}
	}
}
{{- end}}

func Get{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, version meta.Version) (*{{.Name}}, error) {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "get", key.Region, key.Zone, string(version))

	var gceObj interface{}
	var err error
	switch version {
	case meta.VersionAlpha:
		switch key.Type() {
		case meta.Regional:
 			klog.V(3).Infof("Getting alpha region {{.Name}} %v", key.Name)
			gceObj, err = gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Get(ctx, key)
		default:
      klog.V(3).Infof("Getting alpha {{.Name}} %v", key.Name)
			gceObj, err = gceCloud.Compute().AlphaGlobal{{.GetCloudProviderName}}().Get(ctx, key)
		}
	case meta.VersionBeta:
		switch key.Type() {
		case meta.Regional:
      klog.V(3).Infof("Getting beta region {{.Name}} %v", key.Name)
			gceObj, err = gceCloud.Compute().Beta{{.GetCloudProviderName}}().Get(ctx, key)
		default:
      klog.V(3).Infof("Getting beta {{.Name}} %v", key.Name)
			gceObj, err = gceCloud.Compute().BetaGlobal{{.GetCloudProviderName}}().Get(ctx, key)
		}
	default:
		switch key.Type() {
		case meta.Regional:
      klog.V(3).Infof("Getting ga region {{.Name}} %v", key.Name)
			gceObj, err = gceCloud.Compute().{{.GetCloudProviderName}}().Get(ctx, key)
		default:
      klog.V(3).Infof("Getting ga {{.Name}} %v", key.Name)
			gceObj, err = gceCloud.Compute().Global{{.GetCloudProviderName}}().Get(ctx, key)
		}
	}
	if err != nil {
		return nil, mc.Observe(err)
	}
	compositeType, err := To{{.Name}}(gceObj)
  if err != nil {
    return nil, err
  }

	if key.Type() == meta.Regional {
    compositeType.Scope = meta.Regional
  }

  compositeType.Version = version
  return compositeType, nil
}

func List{{.GetCloudProviderName}}(gceCloud *gce.Cloud, key *meta.Key, version meta.Version) ([]*{{.Name}}, error) {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "get", key.Region, key.Zone, string(version))

	var gceObjs interface{}
	var err error
	switch version {
	case meta.VersionAlpha:
		switch key.Type() {
		case meta.Regional:
 			klog.V(3).Infof("Listing alpha region {{.Name}}")
			gceObjs, err = gceCloud.Compute().Alpha{{.GetCloudProviderName}}().List(ctx, key.Region, filter.None)
		default:
 			klog.V(3).Infof("Listing alpha {{.Name}}")
			gceObjs, err = gceCloud.Compute().AlphaGlobal{{.GetCloudProviderName}}().List(ctx, filter.None)
		}
	case meta.VersionBeta:
		switch key.Type() {
		case meta.Regional:
 			klog.V(3).Infof("Listing beta region {{.Name}}")
			gceObjs, err = gceCloud.Compute().Beta{{.GetCloudProviderName}}().List(ctx, key.Region, filter.None)
		default:
 			klog.V(3).Infof("Listing beta {{.Name}}")
			gceObjs, err = gceCloud.Compute().BetaGlobal{{.GetCloudProviderName}}().List(ctx, filter.None)
		}
	default:
		switch key.Type() {
		case meta.Regional:
 			klog.V(3).Infof("Listing ga region {{.Name}}")
			gceObjs, err = gceCloud.Compute().{{.GetCloudProviderName}}().List(ctx, key.Region, filter.None)
		default:
 			klog.V(3).Infof("Listing ga {{.Name}}")
			gceObjs, err = gceCloud.Compute().Global{{.GetCloudProviderName}}().List(ctx, filter.None)
		}
	}
	if err != nil {
		return nil, mc.Observe(err)
	}
	compositeObjs, err := To{{.Name}}List(gceObjs)
	if err != nil {
		return nil, err
	}
	for _, obj := range compositeObjs {
		obj.Version = version
	}
	return compositeObjs, nil
}

func Delete{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, version meta.Version) error {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "delete", key.Region, key.Zone, string(version))

	switch version {
	case meta.VersionAlpha:
		switch key.Type() {
		case meta.Regional:
		  klog.V(3).Infof("Deleting alpha region {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Delete(ctx, key))
		default:
		  klog.V(3).Infof("Deleting alpha {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().AlphaGlobal{{.GetCloudProviderName}}().Delete(ctx, key))
		}
	case meta.VersionBeta:
		switch key.Type() {
		case meta.Regional:
  		klog.V(3).Infof("Deleting beta region {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().Beta{{.GetCloudProviderName}}().Delete(ctx, key))
		default:
		  klog.V(3).Infof("Deleting beta {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().BetaGlobal{{.GetCloudProviderName}}().Delete(ctx, key))
		}
	default:
    switch key.Type() {
		case meta.Regional:
		  klog.V(3).Infof("Deleting ga region {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().{{.GetCloudProviderName}}().Delete(ctx, key))
		default:
		  klog.V(3).Infof("Deleting ga {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().Global{{.GetCloudProviderName}}().Delete(ctx, key))
		}
	}
}

// To{{.Name}}List converts a list of compute alpha, beta or GA
// {{.Name}} into a list of our composite type.
func To{{.Name}}List(objs interface{}) ([]*{{.Name}}, error) {
	result := []*{{.Name}}{}

	err := copyViaJSON(&result, objs)
	if err != nil {
		return nil, fmt.Errorf("Could not copy object %v to list of {{.Name}} via JSON: %v", objs, err)
	}
	return result, nil
}
	
{{ else }}
func Create{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, {{.VarName}} *{{.Name}}) error {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()
  mc := compositemetrics.NewMetricContext("{{.Name}}", "create", key.Region, key.Zone, string({{.VarName}}.Version))

	switch {{.VarName}}.Version {
	case meta.VersionAlpha:
		alpha, err := {{.VarName}}.ToAlpha()
		if err != nil {
			return err
		}
		switch key.Type() {
		case meta.Regional:
  		klog.V(3).Infof("Creating alpha region {{.Name}} %v", alpha.Name)
			alpha.Region = key.Region
			return mc.Observe(gceCloud.Compute().AlphaRegion{{.GetCloudProviderName}}().Insert(ctx, key, alpha))
		default:
  		klog.V(3).Infof("Creating alpha {{.Name}} %v", alpha.Name)
			return mc.Observe(gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Insert(ctx, key, alpha))
	}
	case meta.VersionBeta:
		beta, err := {{.VarName}}.ToBeta()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Creating beta {{.Name}} %v", beta.Name)
		return mc.Observe(gceCloud.Compute().Beta{{.GetCloudProviderName}}().Insert(ctx, key, beta))
	default:
		ga, err := {{.VarName}}.ToGA()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Creating ga {{.Name}} %v", ga.Name)
		return mc.Observe(gceCloud.Compute().{{.GetCloudProviderName}}().Insert(ctx, key, ga))
	}
}

{{if .HasUpdate}}
func Update{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, {{.VarName}} *{{.Name}}) error {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "update", key.Region, key.Zone, string({{.VarName}}.Version))
	switch {{.VarName}}.Version {
	case meta.VersionAlpha:
		alpha, err := {{.VarName}}.ToAlpha()
		if err != nil {
			return err
		}
		switch key.Type() {
		case meta.Regional:
		  klog.V(3).Infof("Updating alpha region {{.Name}} %v", alpha.Name)
			return mc.Observe(gceCloud.Compute().AlphaRegion{{.GetCloudProviderName}}().Update(ctx, key, alpha))
		default:
      klog.V(3).Infof("Updating alpha {{.Name}} %v", alpha.Name)
			return mc.Observe(gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Update(ctx, key, alpha))
		}
	case meta.VersionBeta:
		beta, err := {{.VarName}}.ToBeta()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Updating beta {{.Name}} %v", beta.Name)
		return mc.Observe(gceCloud.Compute().Beta{{.GetCloudProviderName}}().Update(ctx, key, beta))
	default:
		ga, err := {{.VarName}}.ToGA()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Updating ga {{.Name}} %v", ga.Name)
		return mc.Observe(gceCloud.Compute().{{.GetCloudProviderName}}().Update(ctx, key, ga))
	}
}
{{- end}}

func Delete{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, version meta.Version) error {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "delete", key.Region, key.Zone, string(version))

	switch version {
	case meta.VersionAlpha:
		switch key.Type() {
		case meta.Regional:
		  klog.V(3).Infof("Deleting alpha region {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().AlphaRegion{{.GetCloudProviderName}}().Delete(ctx, key))
		default:
		  klog.V(3).Infof("Deleting alpha {{.Name}} %v", key.Name)
			return mc.Observe(gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Delete(ctx, key))
		}
	case meta.VersionBeta:
		klog.V(3).Infof("Deleting beta {{.Name}} %v", key.Name)
		return mc.Observe(gceCloud.Compute().Beta{{.GetCloudProviderName}}().Delete(ctx, key))
	default:
		klog.V(3).Infof("Deleting ga {{.Name}} %v", key.Name)
		return mc.Observe(gceCloud.Compute().{{.GetCloudProviderName}}().Delete(ctx, key))
	}
}

func Get{{.Name}}(gceCloud *gce.Cloud, key *meta.Key, version meta.Version) (*{{.Name}}, error) {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "get", key.Region, key.Zone, string(version))

	var gceObj interface{}
	var err error
	switch version {
	case meta.VersionAlpha:
		switch key.Type() {
			case meta.Regional:
		  	klog.V(3).Infof("Getting alpha region {{.Name}} %v", key.Name)
				gceObj, err = gceCloud.Compute().AlphaRegion{{.GetCloudProviderName}}().Get(ctx, key)
			default:
		  	klog.V(3).Infof("Getting alpha {{.Name}} %v", key.Name)
				gceObj, err = gceCloud.Compute().Alpha{{.GetCloudProviderName}}().Get(ctx, key)
		}
	case meta.VersionBeta:
    klog.V(3).Infof("Getting beta {{.Name}} %v", key.Name)
		gceObj, err = gceCloud.Compute().Beta{{.GetCloudProviderName}}().Get(ctx, key)
	default:
    klog.V(3).Infof("Getting ga {{.Name}} %v", key.Name)
		gceObj, err = gceCloud.Compute().{{.GetCloudProviderName}}().Get(ctx, key)
	}
	if err != nil {
		return nil, mc.Observe(err)
	}
	compositeType, err := To{{.Name}}(gceObj)
  if err != nil {
    return nil, err
  }

  compositeType.Version = version
  return compositeType, nil
}

func List{{.GetCloudProviderName}}(gceCloud *gce.Cloud, key *meta.Key, version meta.Version) ([]*{{.Name}}, error) {
	ctx, cancel := cloudprovider.ContextWithCallTimeout()
	defer cancel()	
  mc := compositemetrics.NewMetricContext("{{.Name}}", "get", key.Region, key.Zone, string(version))

	var gceObjs interface{}
	var err error
	switch version {
	case meta.VersionAlpha:
		switch key.Type() {
			case meta.Regional:
		  	klog.V(3).Infof("Listing alpha region {{.Name}}")
				gceObjs, err = gceCloud.Compute().AlphaRegion{{.GetCloudProviderName}}().List(ctx, key.Region, filter.None)
			default:
		  	klog.V(3).Infof("Listing alpha {{.Name}}")
				gceObjs, err = gceCloud.Compute().Alpha{{.GetCloudProviderName}}().List(ctx, filter.None)
		}
	case meta.VersionBeta:
		  klog.V(3).Infof("Listing beta {{.Name}}")
			gceObjs, err = gceCloud.Compute().Beta{{.GetCloudProviderName}}().List(ctx, filter.None)
	default:
		  klog.V(3).Infof("Listing ga {{.Name}}")
			gceObjs, err = gceCloud.Compute().{{.GetCloudProviderName}}().List(ctx, filter.None)
	}
	if err != nil {
		return nil, mc.Observe(err)
	}

	compositeObjs, err := To{{.Name}}List(gceObjs)
	if err != nil {
		return nil, err
	}
	for _, obj := range compositeObjs {
		obj.Version = version
	}
	return compositeObjs, nil
}

// To{{.Name}}List converts a list of compute alpha, beta or GA
// {{.Name}} into a list of our composite type.
func To{{.Name}}List(objs interface{}) ([]*{{.Name}}, error) {
	result := []*{{.Name}}{}

	err := copyViaJSON(&result, objs)
	if err != nil {
		return nil, fmt.Errorf("could not copy object %v to list of {{.Name}} via JSON: %v", objs, err)
	}
	return result, nil
}

{{- end}}

// To{{.Name}} converts a compute alpha, beta or GA
// {{.Name}} into our composite type.
func To{{.Name}}(obj interface{}) (*{{.Name}}, error) {
	{{.VarName}} := &{{.Name}}{}
	err := copyViaJSON({{.VarName}}, obj)
	if err != nil {
		return nil, fmt.Errorf("could not copy object %+v to {{.Name}} via JSON: %v", obj, err)
	}

	return {{.VarName}}, nil
}

{{- range $version, $extension := $.Versions}}
{{$lower := $version | ToLower}}
// To{{$version}} converts our composite type into an {{$lower}} type.
// This {{$lower}} type can be used in GCE API calls.
func ({{$type.VarName}} *{{$type.Name}}) To{{$version}}() (*compute{{$extension}}.{{$type.Name}}, error) {
	{{$lower}} := &compute{{$extension}}.{{$type.Name}}{}
	err := copyViaJSON({{$lower}}, {{$type.VarName}})
	if err != nil {
		return nil, fmt.Errorf("error converting {{$type.Name}} to compute {{$lower}} type via JSON: %v", err)
	}

	{{- if eq $type.Name "BackendService"}}
	// Set force send fields. This is a temporary hack.
	if {{$lower}}.CdnPolicy != nil && {{$lower}}.CdnPolicy.CacheKeyPolicy != nil {
		{{$lower}}.CdnPolicy.CacheKeyPolicy.ForceSendFields = []string{"IncludeHost", "IncludeProtocol", "IncludeQueryString", "QueryStringBlacklist", "QueryStringWhitelist"}
	}
	if {{$lower}}.Iap != nil {
		{{$lower}}.Iap.ForceSendFields = []string{"Enabled", "Oauth2ClientId", "Oauth2ClientSecret"}
	}
	{{- end}}	

	return {{$lower}}, nil
}
{{- end}} {{/* range versions */}}
{{- end}} {{/* isMainType */}}
{{- end}} {{/* range */}}
`
	data := struct {
		All      []compositemeta.ApiService
		Versions map[string]string
	}{compositemeta.AllApiServices, compositemeta.Versions}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	tmpl := template.Must(template.New("funcs").Funcs(funcMap).Parse(text))
	if err := tmpl.Execute(wr, data); err != nil {
		panic(err)
	}
}

// genTests() generates all of the tests
func genTests(wr io.Writer) {
	const text = `
{{ $All := .All}}
{{range $type := $All}}
		{{- if .IsMainService}}
			func Test{{.Name}}(t *testing.T) {
	// Use reflection to verify that our composite type contains all the
	// same fields as the alpha type.
	compositeType := reflect.TypeOf({{.Name}}{})
	alphaType := reflect.TypeOf(computealpha.{{.Name}}{})
	betaType := reflect.TypeOf(computebeta.{{.Name}}{})
	gaType := reflect.TypeOf(compute.{{.Name}}{})

	// For the composite type, remove the Version field from consideration
	compositeTypeNumFields := compositeType.NumField() - 2
	if compositeTypeNumFields != alphaType.NumField() {
		t.Fatalf("%v should contain %v fields. Got %v", alphaType.Name(), alphaType.NumField(), compositeTypeNumFields)
	}

  // Compare all the fields by doing a lookup since we can't guarantee that they'll be in the same order
	// Make sure that composite type is strictly alpha fields + internal bookkeeping
	for i := 2; i < compositeType.NumField(); i++ {
		lookupField, found := alphaType.FieldByName(compositeType.Field(i).Name)
		if !found {
			t.Fatal(fmt.Errorf("Field %v not present in alpha type %v", compositeType.Field(i), alphaType))
		}
		if err := compareFields(compositeType.Field(i), lookupField); err != nil {
			t.Fatal(err)
		}
	}

  // Verify that all beta fields are in composite type
	if err := typeEquality(betaType, compositeType, false); err != nil {
		t.Fatal(err)
	}

	// Verify that all GA fields are in composite type
	if err := typeEquality(gaType, compositeType, false); err != nil {
		t.Fatal(err)
	}
}

func TestTo{{.Name}}(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected *{{.Name}}
	}{
		{
			computealpha.{{.Name}}{},
			&{{.Name}}{},
		},
		{
			computebeta.{{.Name}}{},
			&{{.Name}}{},
		},
		{
			compute.{{.Name}}{},
			&{{.Name}}{},
		},
	}
	for _, testCase := range testCases {
		result, _ := To{{.Name}}(testCase.input)
		if !reflect.DeepEqual(result, testCase.expected) {
			t.Fatalf("To{{.Name}}(input) = \ninput = %s\n%s\nwant = \n%s", pretty.Sprint(testCase.input), pretty.Sprint(result), pretty.Sprint(testCase.expected))
		}
	}
}

{{range $version, $extension := $.Versions}}
func Test{{$type.Name}}To{{$version}}(t *testing.T) {
	composite := {{$type.Name}}{}
	expected := &compute{{$extension}}.{{$type.Name}}{}
	result, err := composite.To{{$version}}()
	if err != nil {
		t.Fatalf("{{$type.Name}}.To{{$version}}() error: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("{{$type.Name}}.To{{$version}}() = \ninput = %s\n%s\nwant = \n%s", pretty.Sprint(composite), pretty.Sprint(result), pretty.Sprint(expected))
	}
}
{{- end}}
{{- else}}

func Test{{.Name}}(t *testing.T) {
	compositeType := reflect.TypeOf({{.Name}}{})
	alphaType := reflect.TypeOf(computealpha.{{.Name}}{})
	if err := typeEquality(compositeType, alphaType, true); err != nil {
		t.Fatal(err)
	}
}
{{- end}}
{{- end}}
`
	data := struct {
		All      []compositemeta.ApiService
		Versions map[string]string
	}{compositemeta.AllApiServices, compositemeta.Versions}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	tmpl := template.Must(template.New("tests").Funcs(funcMap).Parse(text))
	if err := tmpl.Execute(wr, data); err != nil {
		panic(err)
	}
}

func main() {
	out := &bytes.Buffer{}
	testOut := &bytes.Buffer{}

	genHeader(out)
	genTypes(out)
	genFuncs(out)

	genTestHeader(testOut)
	genTests(testOut)

	var err error
	err = ioutil.WriteFile("./pkg/composite/gen.go", []byte(gofmtContent(out)), 0644)
	//err = ioutil.WriteFile("./pkg/composite/composite.go", []byte(out.String()), 0644)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile("./pkg/composite/gen_test.go", []byte(gofmtContent(testOut)), 0644)
	if err != nil {
		panic(err)
	}
}
