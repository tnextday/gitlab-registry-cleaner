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
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"

	"github.com/spf13/pflag"
)

const version = "v0.1"

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
	flags           = pflag.NewFlagSet("", pflag.ExitOnError)
	durationRegex   = regexp.MustCompile(`(\d+)]\s*([a-z]+)`)
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
	log.Printf(format, v...)
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
	log.Printf(`Usage of gitlab-registry-cleaner (%s)

optional arguments:
`, version)
	flags.PrintDefaults()
}

func main() {
	pflag.ErrHelp = nil
	flags.Usage = usage
	flags.StringVar(&gitlabToken, "token", "", "gitlab private token, environment: GITLAB_TOKEN")
	flags.StringVar(&gitlabBaseUrl, "base-url", "https://gitlab.com/", "gitlab base url, environment: GITLAB_BASE_URL")
	flags.StringVar(&projectId, "project-id", "", "the ID or path of the project, environment: GITLAB_PROJECT_ID")
	flags.StringArrayVarP(&registriesRegex, "registry", "r", []string{}, "registry repositories regex list, clean all repositories in project if registry not set")
	flags.StringArrayVarP(&tagsRegex, "tag", "t", []string{}, "image tag regex list")
	flags.StringArrayVarP(&excludesRegex, "exclude", "e", []string{}, "exclude image tag regex list")
	flags.IntVarP(&keepsN, "keep-n", "k", 0, "keeps N latest matching tagsRegex for each registry repositories")
	flags.StringVarP(&olderThen, "older-then", "o", "", "Tags to delete that are older than the given time, written in human readable form 1h, 1d, 1m.")
	flags.BoolVarP(&dryRun, "dry-run", "n", false, "only print which images would be deleted")
	flags.BoolVarP(&insecure, "insecure", "K", false, "allow insecure connections over plain HTTP")
	flags.BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	flags.SortFlags = false
	if err := flags.Parse(os.Args); err != nil {
		log.Printf("Parse args error: %v\n", err)
		os.Exit(1)
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
			log.Printf("Project ID should't empty\n")
			os.Exit(1)
		}
	}

	olderThenDuration, err := parserDuration(olderThen)
	if err != nil {
		log.Println(err)
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
			log.Printf("Compile regex %s error: %v\n", rs, err)
			os.Exit(1)
		}
	}

	for _, rs := range tagsRegex {
		if regx, err := regexp.Compile(rs); err == nil {
			tagRegs = append(tagRegs, regx)
		} else {
			log.Printf("Compile regex %s error: %v\n", rs, err)
			os.Exit(1)
		}
	}
	for _, rs := range excludesRegex {
		if regx, err := regexp.Compile(rs); err == nil {
			excludeRegs = append(excludeRegs, regx)
		} else {
			log.Printf("Compile regex %s error: %v\n", rs, err)
			os.Exit(1)
		}
	}

	gl := gitlab.NewClient(nil, gitlabToken)
	if err := gl.SetBaseURL(gitlabBaseUrl); err != nil {
		log.Printf("Set gitlab base url error: %v\n", err)
		os.Exit(1)
	}
	repos, _, err := gl.ContainerRegistry.ListRegistryRepositories(projectId, nil)
	if err != nil {
		log.Printf("List registry repositories error: %v\n", err)
		os.Exit(1)
	}
	var matchedRepos []*gitlab.RegistryRepository
	for _, repo := range repos {
		if len(registryRegs) == 0 || !matchRegexList(repo.Name, registryRegs) {
			verboseLogf("Skipped registry repository: %v\n", repo.Name)
		} else {
			verboseLogf("Matched registry repository: %v\n", repo.Name)
			matchedRepos = append(matchedRepos, repo)
		}
	}

	if len(matchedRepos) == 0 {
		log.Println("There is no registry repository found.")
		os.Exit(1)
	}

	for _, repo := range matchedRepos {
		log.Printf("Searching in %v\n", repo.Path)
		tags, _, err := gl.ContainerRegistry.ListRegistryRepositoryTags(projectId, repo.ID, nil)
		if err != nil {
			log.Printf("List registry repository tags failed, path: %s, error: %v\n", repo.Path, err)
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
			if len(tagRegs) == 0 || !matchRegexList(tag.Name, tagRegs) {
				verboseLogf("Skipped tag: %v\n", tag.Name)
			} else {
				verboseLogf("Matched tag: %v\n", tag.Name)
				matchedTags = append(matchedTags, tag)
			}
		}
		if keepsN > 0 && len(matchedTags) <= keepsN {
			log.Printf("Skip because of less mathced tags(%v) then keeps N(%v)\n", len(matchedTags), keepsN)
			continue
		}
		for _, tag := range matchedTags {
			t, _, err := gl.ContainerRegistry.GetRegistryRepositoryTagDetail(projectId, repo.ID, tag.Name)
			if err != nil {
				log.Printf("Get registry repository tag detail failed, path: %s, error: %v\n", tag.Path, err)
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
				log.Printf("[Dry run]%s will be delete\n", t.Path)
				continue
			}
			log.Printf("Delete %s ", t.Path)
			_, err := gl.ContainerRegistry.DeleteRegistryRepositoryTag(projectId, repo.ID, t.Name)
			if err == nil {
				deletedCount++
				log.Println("OK")
			} else {
				log.Println("error:", err)
			}
		}
		log.Printf("%v/%v tags have been deleted in %v\n", deletedCount, len(tagsToDelete), repo.Path)
	}
}