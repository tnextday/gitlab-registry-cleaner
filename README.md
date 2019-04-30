gitlab-registry-cleaner
=======================

[简体中文](README_ZH.md)

gitlab-registry-cleaner is a tool for batch cleaning gitlab image repositories. It supports multiple ways to match the images that need to be cleaned up, and is easy to integrate into gitlab CI.

# Usage

```
Usage of gitlab-registry-cleaner

Options:
  -T, --token string           Gitlab private token, environment: GITLAB_TOKEN
      --base-url string        Gitlab base url, environment: GITLAB_BASE_URL (default "https://gitlab.com/")
  -p, --project string         [REQUIRED]The ID or path of the project, environment: GITLAB_PROJECT_ID
  -r, --registry stringArray   Registry repository path regex list, clean all repositories in project if registry not set
  -t, --tag stringArray        Image tag regex list
  -e, --exclude stringArray    Exclude image tag regex list
  -n, --keep-n int             Keeps N latest matching tagsRegex for each registry repositories (default 10)
  -o, --older-then string      Tags to delete that are older than the given time, written in human readable form 1h, 1d, 1m
  -d, --dry-run                Only print which images would be deleted
  -k, --insecure               Allow connections to SSL sites without certs
  -v, --verbose                Verbose output
  -V, --version                Print version and exit
  -h, --help                   Print help and exit

```

# Example

Clean up the image in the `foo/bar` project for more than one month except the version number, but at least keep the last 5

```
export GITLAB_TOKEN=<gitlab-private-token>
gitlab-registry-cleaner -p foo/bar --older-then 1month --keep-n 5 --exclude '^v?[0-9.]+$'
```

Please refer to [`.gitlab-ci.yml`](.gitlab-ci.yml) for use in CI.