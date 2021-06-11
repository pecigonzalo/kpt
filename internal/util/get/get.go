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

// Package get contains libraries for fetching packages.
package get

import (
	"bytes"
	goerrors "errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/kpt/internal/gitutil"
	"github.com/GoogleContainerTools/kpt/internal/util/git"
	"github.com/GoogleContainerTools/kpt/pkg/kptfile"
	"github.com/GoogleContainerTools/kpt/pkg/kptfile/kptfileutil"
	"sigs.k8s.io/kustomize/kyaml/copyutil"
	"sigs.k8s.io/kustomize/kyaml/errors"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// Command fetches a package from a git repository and copies it to a local directory.
type Command struct {
	// Git contains information about the git repo to fetch
	kptfile.Git

	// Destination is the output directory to clone the package to.  Defaults to the name of the package --
	// either the base repo name, or the base subdirectory name.
	Destination string

	// Name is the name to give the package.  Defaults to the destination.
	Name string

	// Remove directory before copying to it.
	Clean bool
}

// Run runs the Command.
func (c Command) Run() error {
	if err := (&c).DefaultValues(); err != nil {
		return err
	}

	if _, err := os.Stat(c.Destination); !c.Clean && !os.IsNotExist(err) {
		return errors.Errorf("destination directory %q already exists", c.Destination)
	}

	// normalize path to a filepath
	if !strings.HasSuffix(c.Directory, "file://") {
		c.Directory = filepath.Join(path.Split(c.Directory))
	}

	// define where we are going to clone the package from
	r := &git.RepoSpec{OrgRepo: c.Repo, Path: c.Directory, Ref: c.Ref}

	defaultRef, err := gitutil.DefaultRef(c.Repo)
	if err != nil {
		return err
	}

	// clone the repo to a tmp directory.
	// delete the tmp directory later.
	err = ClonerUsingGitExec(r, defaultRef)
	if err != nil {
		return errors.Errorf("failed to clone git repo: %v", err)
	}
	defer os.RemoveAll(r.Dir)

	// delete the existing package if it exists
	if c.Clean {
		err = os.RemoveAll(c.Destination)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	// copy the git sub directory to the destination
	err = copyutil.CopyDir(r.AbsPath(), c.Destination)
	if err != nil {
		return errors.WrapPrefixf(err, "missing subdirectory %q in repo %q at ref %q\n",
			r.Path, r.OrgRepo, r.Ref)
	}

	// create or update the KptFile with the values from git
	if err = (&c).upsertKptfile(r); err != nil {
		return errors.Wrap(err)
	}
	return nil
}

// Cloner is a function that can clone a git repo.
type Cloner func(repoSpec *git.RepoSpec) error

// ClonerUsingGitExec uses a local git install, as opposed
// to say, some remote API, to obtain a local clone of
// a remote repo.
func ClonerUsingGitExec(repoSpec *git.RepoSpec, defaultRef string) error {
	// look for a tag with the directory as a prefix for versioning
	// subdirectories independently
	originalRef := repoSpec.Ref
	if repoSpec.Path != "" && !strings.Contains(repoSpec.Ref, "refs") {
		// join the directory with the Ref (stripping the preceding '/' if it exists)
		repoSpec.Ref = path.Join(strings.TrimLeft(repoSpec.Path, "/"), repoSpec.Ref)
	}

	// clone the repo to a tmp directory.
	// delete the tmp directory later.
	err := clonerUsingGitExec(repoSpec)
	if err != nil && originalRef != repoSpec.Ref {
		repoSpec.Ref = originalRef
		err = clonerUsingGitExec(repoSpec)
	}

	if err != nil {
		if strings.HasPrefix(repoSpec.Path, "blob/") {
			return errors.Errorf("failed to clone git repo containing /blob/, "+
				"you may need to remove /blob/%s from the url:\n%v", defaultRef, err)
		}
		return errors.Errorf("failed to clone git repo: %v", err)
	}

	// Read the Kptfile in the package to make sure it is using the correct
	// version of the Kptfile resource.
	if _, err := kptfileutil.ReadFile(filepath.Join(repoSpec.Dir, repoSpec.Path)); err != nil {
		// Packages might not have a Kptfile, and that is ok.
		if goerrors.Is(err, os.ErrNotExist) {
			return nil
		}

		// Update the error with the repoSpec information so we don't end up
		// printing the path to a Kptfile in some temporary directory.
		var unknownKptfileVersionError *kptfileutil.UnknownKptfileVersionError
		if goerrors.As(err, &unknownKptfileVersionError) {
			unknownKptfileVersionError.RepoSpec = repoSpec
		}
		return err
	}

	return nil
}

func clonerUsingGitExec(repoSpec *git.RepoSpec) error {
	gitProgram, err := exec.LookPath("git")
	if err != nil {
		return errors.WrapPrefixf(err, "no 'git' program on path")
	}

	repoSpec.Dir, err = ioutil.TempDir("", "kpt-get-")
	if err != nil {
		return err
	}
	cmd := exec.Command(gitProgram, "init", repoSpec.Dir)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing empty git repo: %s", out.String())
		return errors.WrapPrefixf(err, "trouble initializing empty git repo in %q",
			repoSpec.Dir)
	}

	cmd = exec.Command(gitProgram, "remote", "add", "origin", repoSpec.CloneSpec())
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.Dir = repoSpec.Dir
	err = cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting git remote: %s", out.String())
		return errors.WrapPrefixf(
			err,
			"trouble adding remote %q",
			repoSpec.CloneSpec())
	}
	if repoSpec.Ref == "" {
		repoSpec.Ref, err = gitutil.DefaultRef(repoSpec.Dir)
		if err != nil {
			return err
		}
	}

	err = func() error {
		cmd = exec.Command(gitProgram, "fetch", "origin", "--depth=1", repoSpec.Ref)
		cmd.Stdout = &out
		cmd.Stderr = &out
		cmd.Dir = repoSpec.Dir
		err = cmd.Run()
		if err != nil {
			return errors.WrapPrefixf(err, "trouble fetching %q, "+
				"please run 'git clone <REPO>; stat <DIR/SUBDIR>' to verify credentials", repoSpec.Ref)
		}
		cmd = exec.Command(gitProgram, "reset", "--hard", "FETCH_HEAD")
		cmd.Stdout = &out
		cmd.Stderr = &out
		cmd.Dir = repoSpec.Dir
		err = cmd.Run()
		if err != nil {
			return errors.WrapPrefixf(
				err, "trouble hard resetting empty repository to %q", repoSpec.Ref)
		}
		return nil
	}()
	if err != nil {
		cmd = exec.Command(gitProgram, "fetch", "origin")
		cmd.Stdout = &out
		cmd.Stderr = &out
		cmd.Dir = repoSpec.Dir
		if err = cmd.Run(); err != nil {
			return errors.WrapPrefixf(err, "trouble fetching origin, "+
				"please run 'git clone <REPO>; stat <DIR/SUBDIR>' to verify credentials")
		}
		cmd = exec.Command(gitProgram, "reset", "--hard", repoSpec.Ref)
		cmd.Stdout = &out
		cmd.Stderr = &out
		cmd.Dir = repoSpec.Dir
		if err = cmd.Run(); err != nil {
			return errors.WrapPrefixf(
				err, "trouble hard resetting empty repository to %q, "+
					"please run 'git clone <REPO>; stat <DIR/SUBDIR>' to verify credentials", repoSpec.Ref)
		}
	}

	cmd = exec.Command(gitProgram, "submodule", "update", "--init", "--recursive")
	cmd.Stdout = &out
	cmd.Dir = repoSpec.Dir
	err = cmd.Run()
	if err != nil {
		return errors.WrapPrefixf(err, "trouble fetching submodules for %q, "+
			"please run 'git clone <REPO>; stat <DIR/SUBDIR>' to verify credentials", repoSpec.Ref)
	}

	return nil
}

// DefaultValues sets values to the default values if they were unspecified
func (c *Command) DefaultValues() error {
	if len(c.Repo) == 0 {
		return errors.Errorf("must specify repo")
	}
	if len(c.Ref) == 0 {
		return errors.Errorf("must specify ref")
	}
	if len(c.Destination) == 0 {
		return errors.Errorf("must specify destination")
	}
	if len(c.Directory) == 0 {
		return errors.Errorf("must specify remote subdirectory")
	}

	// default the name to the destination name
	if len(c.Name) == 0 {
		c.Name = filepath.Base(c.Destination)
	}

	return nil
}

// upsertKptfile populates the KptFile values, merging any cloned KptFile and the
// cloneFrom values.
func (c *Command) upsertKptfile(spec *git.RepoSpec) error {
	// read KptFile cloned with the package if it exists
	kpgfile, err := kptfileutil.ReadFile(c.Destination)
	if err != nil {
		// no KptFile present, create a default
		kpgfile = kptfile.KptFile{
			ResourceMeta: yaml.ResourceMeta{
				TypeMeta: yaml.TypeMeta{
					APIVersion: kptfile.TypeMeta.APIVersion,
					Kind:       kptfile.TypeMeta.Kind,
				},
				ObjectMeta: yaml.ObjectMeta{
					NameMeta: yaml.NameMeta{
						Name: c.Name,
					},
				},
			},
		}
	}

	// find the git commit sha that we cloned the package at so we can write it to the KptFile
	cmd := exec.Command("git", "rev-parse", "--verify", "HEAD")
	cmd.Dir = spec.AbsPath()
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(b))

	// populate the cloneFrom values so we know where the package came from
	kpgfile.Upstream = kptfile.Upstream{
		Type: kptfile.GitOrigin,
		Git:  c.Git,
	}
	kpgfile.Upstream.Git.Commit = commit
	return kptfileutil.WriteFile(c.Destination, kpgfile)
}
