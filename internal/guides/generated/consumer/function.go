// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by "mdtogo"; DO NOT EDIT.
package consumer

var FunctionGuide = `
## Functions Explained

KRM Config Functions are client-side programs that make it easy to operate on a
package of Kubernetes configuration files.

In GitOps workflows, KPT functions read and write configuration files from a
Git repo. Changes to the system authored by humans and mutating KPT functions
are reviewed before being committed to the repo. They can be run locally or as
part of a CI/CD pipeline, or as pre-commit or post-commit steps to validate
configurations before they are applied to a cluster.

Kpt offers multiple runtimes for configuration functions to run arbitrary
actions on configuration. By default kpt runs configuration functions in a
[container runtime], but it also provides runtimes for functions packaged as
[executables] or [starlark scripts].

The image below details how a function adds the label ` + "`" + `color: pink` + "`" + ` to a
package.

{{< svg src="images/fn" >}}

Functions can address many types of use cases, including:

- **Configuration Validation:** e.g. Lint Kubernetes resources using ` + "`" + `kubeval` + "`" + `.
- **Configuration Generation:** e.g. Generate configuration using ` + "`" + `kustomize` + "`" + `.
- **Configuration Transformation:** e.g. Upsert Helm chart configuration to an
  existing package of hydrated helm configs using the latest version of the
  chart.

## Running Functions

Functions may be run either imperatively using the form
` + "`" + `kpt fn run DIR/ --image some-image:version` + "`" + `, or they may be run declaratively
using the form ` + "`" + `kpt fn run DIR/` + "`" + `.

Either way, ` + "`" + `kpt fn run` + "`" + ` will

1. read the package directory ` + "`" + `DIR/` + "`" + ` as input
2. encapsulate the package resources in a ` + "`" + `ResourceList` + "`" + `
3. run the function(s), providing the ResourceList as input
4. write the function(s) output back to the package directory; creating,
   deleting, or updating resources

### Imperative Run

Functions can be run imperatively by specifying the ` + "`" + `--image` + "`" + ` flag.

**Example:** Locally run the container image
` + "`" + `gcr.io/kpt-functions/label-namespace` + "`" + ` against the resources in ` + "`" + `.` + "`" + `.

Let’s look at the example of imperatively running a function to set a label
value. The ([label-namespace]) container image contains a program which adds a
label to all Namespace resources provided to it.

Run the function:

  kpt fn run . --image gcr.io/kpt-functions/label-namespace -- label_name=color label_value=orange

Arguments specified after ` + "`" + `--` + "`" + ` will be provided to the function as a
` + "`" + `ConfigMap` + "`" + ` input containing ` + "`" + `data: {label_name: color, label_value: orange}` + "`" + `.
This is used to parameterize the behavior of the function.

If the package directory ` + "`" + `.` + "`" + ` is not specified, the source will default to STDIN
and sink will default to STDOUT.

**Example:** This is equivalent to the preceding example

  kpt source . |
    kpt fn run --image gcr.io/kpt-functions/label-namespace -- label_name=color label_value=orange |
    kpt sink .

The above example commands will:

- read all resources from the package directory ` + "`" + `.` + "`" + ` to generate input resources
- parse the arguments into a functionConfig field along with input resources
- create a container from the image
- provide the input to the function (container)
- write the output items back to the package directory ` + "`" + `.` + "`" + `

### Declarative Run

Functions and their input configuration may be declared in files rather than
directly on the command line. The declarative method will be the most common
way of invoking config functions in production. Functions can be specified
declaratively using the ` + "`" + `config.kubernetes.io/function` + "`" + ` annotation on a
resource serving as the functionConfig.

**Example:** Equivalent to the imperative run example

We can run the same [label-namespace] example declaratively, which means we
make a reusable function configuration resource which contains all information
necessary to run the function, from container image to argument values. Once we
create file with this information we can check it into [VCS] and run the
function in a repeatable fashion, making it incredibly powerful for production
use.

Create a file ` + "`" + `label-ns-fc.yaml` + "`" + `:

  apiVersion: v1
  kind: ConfigMap
  metadata:
    annotations:
      config.kubernetes.io/function: |
        container:
          image: gcr.io/kpt-functions/label-namespace
  data:
    "label_name": "color"
    "label_value": "orange"

This file contains a ` + "`" + `config.kubernetes.io/function` + "`" + ` annotation specifying the
docker image to use for the config as well as a data field containing a
key-value map with the "label_name" and "label_value" arguments specified
earlier. Using a map also makes it easier to pass more complex arguments values
like a list of strings.

Run the function:

  kpt fn run .

The example command will:

- read all resources from the package directory ` + "`" + `.` + "`" + ` to generate input resources
- for each resource with the ` + "`" + `config.kubernetes.io/function` + "`" + ` annotation, e.g.
  ` + "`" + `label-ns-fc.yaml` + "`" + `, kpt will run the specified function (using the resource
  as the functionConfig)
  - functions are run sequentially, with the output of each function provided
    as input to the next
- write the output items back to the package directory ` + "`" + `.` + "`" + `

Here, rather than specifying ` + "`" + `gcr.io/kpt-functions/label-namespace` + "`" + ` using the
` + "`" + `--image` + "`" + ` flag, we specify it in a file using the
` + "`" + `config.kubernetes.io/function` + "`" + ` annotation.

## Next Steps

- See more examples of functions in the [functions catalog].
- Get a quickstart on writing functions from the [function producer docs].
- Find out how to structure a pipeline of functions from the
  [functions concepts] page.
- Learn more ways of using the ` + "`" + `kpt fn` + "`" + ` command from the [reference] doc.
`