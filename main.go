// Copyright © 2019 tnextday <fw2k4@163.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/spf13/pflag"
)

var (
	gitlabToken     string
	gitlabBaseUrl   string
	projectId       string
	registriesRegex []string
	tagsRegex       []string
	excludesRegex   []string
	keepsN          int
	olderThen       string
	insecure        bool
	dryRun          bool
	verbose         bool
	printVersion    bool
	help            bool
	durationRegex   = regexp.MustCompile(`(\d+)\s*([a-z]+)`)
	httpClient      *http.Client

	AppVersion = "dev"
	BuildTime  = ""
)

func parserDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	ss := durationRegex.FindStringSubmatch(strings.ToLower(s))
	if len(ss) == 0 {
		return 0, errors.New("can't parser the duration string")
	}
	i, _ := strconv.Atoi(ss[1])
	switch ss[2][:1] {
	case "h":
		return time.Hour * time.Duration(i), nil
	case "d":
		return time.Hour * 24 * time.Duration(i), nil
	case "m":
		return time.Hour * 24 * 30 * time.Duration(i), nil
	default:
		return 0, fmt.Errorf("unsupport duration unit: %s", ss[2])
	}
}

func verboseLogf(format string, v ...interface{}) {
	if !verbose {
		return
	}
	fmt.Printf(format, v...)
}

func matchRegexList(s string, list []*regexp.Regexp) bool {
	for _, r := range list {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}

func usage() {
	fmt.Printf(`Usage of gitlab-registry-cleaner

Options:
`)
	pflag.PrintDefaults()
	os.Exit(0)
}

func main() {
	pflag.ErrHelp = nil
	pflag.Usage = usage
	pflag.StringVarP(&gitlabToken, "token", "T", "", "Gitlab private token, environment: GITLAB_TOKEN")
	pflag.StringVar(&gitlabBaseUrl, "base-url", "https://gitlab.com/", "Gitlab base url, environment: GITLAB_BASE_URL")
	pflag.StringVarP(&projectId, "project", "p", "", "[REQUIRED]The ID or path of the project, environment: GITLAB_PROJECT_ID")
	pflag.StringArrayVarP(&registriesRegex, "registry", "r", []string{}, "Registry repository path regex list, clean all repositories in project if registry not set")
	pflag.StringArrayVarP(&tagsRegex, "tag", "t", []string{}, "Image tag regex list")
	pflag.StringArrayVarP(&excludesRegex, "exclude", "e", []string{}, "Exclude image tag regex list")
	pflag.IntVarP(&keepsN, "keep-n", "k", 10, "Keeps N latest matching tagsRegex for each registry repositories")
	pflag.StringVarP(&olderThen, "older-then", "o", "", "Tags to delete that are older than the given time, written in human readable form 1h, 1d, 1m")
	pflag.BoolVarP(&dryRun, "dry-run", "n", false, "Only print which images would be deleted")
	pflag.BoolVarP(&insecure, "insecure", "K", false, "Allow connections to SSL sites without certs")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	pflag.BoolVarP(&printVersion, "version", "V", false, "Print version and exit")
	pflag.BoolVarP(&help, "help", "h", false, "Print help and exit")
	pflag.CommandLine.SortFlags = false
	pflag.Parse()

	if printVersion {
		fmt.Println("App Version:", AppVersion)
		fmt.Println("Go Version:", runtime.Version())
		fmt.Println("Build Time:", BuildTime)
		os.Exit(0)
	}

	if help {
		usage()
	}

	if gitlabToken == "" {
		if env := os.Getenv("GITLAB_TOKEN"); env != "" {
			gitlabToken = env
		}
	}

	if gitlabBaseUrl == "https://gitlab.com/" {
		if env := os.Getenv("GITLAB_BASE_URL"); env != "" {
			gitlabBaseUrl = env
		}
	}

	if projectId == "" {
		if env := os.Getenv("GITLAB_PROJECT_ID"); env != "" {
			projectId = env
		} else {
			fmt.Println("Project ID required!")
			fmt.Printf("try '%s -h' for more information\n", os.Args[0])
			os.Exit(1)
		}
	}

	olderThenDuration, err := parserDuration(olderThen)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	verboseLogf("Gitlab base url: %v\n", gitlabBaseUrl)
	if gitlabToken != "" {
		verboseLogf("Gitlab private token: **HIDDEN**\n")
	}
	verboseLogf("Gitlab project ID: %v\n", projectId)
	if olderThen != "" {
		verboseLogf("Older then duration: %v\n", olderThenDuration)
	}

	var (
		registryRegs []*regexp.Regexp
		tagRegs      []*regexp.Regexp
		excludeRegs  []*regexp.Regexp
	)

	for _, rs := range registriesRegex {
		if regx, err := regexp.Compile(rs); err == nil {
			registryRegs = append(registryRegs, regx)
		} else {
			fmt.Printf("Compile regex %s error: %v\n", rs, err)
			os.Exit(1)
		}
	}

	for _, rs := range tagsRegex {
		if regx, err := regexp.Compile(rs); err == nil {
			tagRegs = append(tagRegs, regx)
		} else {
			fmt.Printf("Compile regex %s error: %v\n", rs, err)
			os.Exit(1)
		}
	}
	for _, rs := range excludesRegex {
		if regx, err := regexp.Compile(rs); err == nil {
			excludeRegs = append(excludeRegs, regx)
		} else {
			fmt.Printf("Compile regex %s error: %v\n", rs, err)
			os.Exit(1)
		}
	}
	if insecure {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient = &http.Client{Transport: tr}
	}

	gl := gitlab.NewClient(httpClient, gitlabToken)
	if err := gl.SetBaseURL(gitlabBaseUrl); err != nil {
		fmt.Printf("Set gitlab base url error: %v\n", err)
		os.Exit(1)
	}
	repos, _, err := gl.ContainerRegistry.ListRegistryRepositories(projectId, nil)
	if err != nil {
		fmt.Printf("List registry repositories error: %v\n", err)
		os.Exit(1)
	}
	var matchedRepos []*gitlab.RegistryRepository
	for _, repo := range repos {
		if len(registryRegs) > 0 && !matchRegexList(repo.Path, registryRegs) {
			verboseLogf("Skipped registry repository: %v\n", repo.Path)
		} else {
			verboseLogf("Matched registry repository: %v\n", repo.Path)
			matchedRepos = append(matchedRepos, repo)
		}
	}

	if len(matchedRepos) == 0 {
		fmt.Println("There is no registry repository found.")
		os.Exit(1)
	}

	for _, repo := range matchedRepos {
		fmt.Printf("Searching in %v\n", repo.Path)
		opt := &gitlab.ListRegistryRepositoryTagsOptions{PerPage: 10000}
		tags, _, err := gl.ContainerRegistry.ListRegistryRepositoryTags(projectId, repo.ID, opt)
		if err != nil {
			fmt.Printf("List registry repository tags failed, path: %s, error: %v\n", repo.Path, err)
			continue
		}
		var matchedTags []*gitlab.RegistryRepositoryTag

		for _, tag := range tags {
			if tag.Name == "latest" {
				verboseLogf("Skipped the latest tag\n")
				continue
			}

			if len(excludeRegs) > 0 && matchRegexList(tag.Name, excludeRegs) {
				verboseLogf("Skipped tag because of exclude rule: %v\n", tag.Name)
				continue
			}
			if len(tagRegs) > 0 && !matchRegexList(tag.Name, tagRegs) {
				verboseLogf("Skipped tag: %v\n", tag.Name)
			} else {
				verboseLogf("Matched tag: %v\n", tag.Name)
				matchedTags = append(matchedTags, tag)
			}
		}
		if keepsN > 0 && len(matchedTags) <= keepsN {
			fmt.Printf("Skip because of less mathced tags(%v) then keeps N(%v)\n", len(matchedTags), keepsN)
			continue
		}
		for _, tag := range matchedTags {
			t, _, err := gl.ContainerRegistry.GetRegistryRepositoryTagDetail(projectId, repo.ID, tag.Name)
			if err != nil {
				fmt.Printf("Get registry repository tag detail failed, path: %s, error: %v\n", tag.Path, err)
				continue
			}
			tag.CreatedAt = t.CreatedAt

		}
		sort.Slice(matchedTags, func(i, j int) bool {
			return matchedTags[i].CreatedAt.After(*matchedTags[j].CreatedAt)
		})
		verboseLogf("Found %v matched tags in %v\n", len(matchedTags), repo.Path)

		if keepsN > 0 {
			verboseLogf("The latest %v matched tags will be keeps\n", keepsN)
			matchedTags = matchedTags[keepsN:]
		}
		var tagsToDelete []*gitlab.RegistryRepositoryTag
		if olderThenDuration > 0 {
			now := time.Now()
			for _, t := range matchedTags {
				createDuration := now.Sub(*t.CreatedAt)
				if createDuration > olderThenDuration {
					tagsToDelete = append(tagsToDelete, t)
				} else {
					verboseLogf("Tag %v will be keep because of it's create only %v\n", t.Name, createDuration)
				}
			}
		} else {
			tagsToDelete = matchedTags
		}

		verboseLogf("%v tags in %v will be delete\n", len(tagsToDelete), repo.Path)

		deletedCount := 0
		for _, t := range tagsToDelete {
			if dryRun {
				fmt.Printf("[Dry run]%s will be delete\n", t.Path)
				continue
			}
			fmt.Printf("Delete %s ", t.Path)
			_, err := gl.ContainerRegistry.DeleteRegistryRepositoryTag(projectId, repo.ID, t.Name)
			if err == nil {
				deletedCount++
				fmt.Println("OK")
			} else {
				fmt.Println("error:", err)
			}
		}
		fmt.Printf("%v/%v tags have been deleted in %v\n", deletedCount, len(tagsToDelete), repo.Path)
	}
}
