gitlab-registry-cleaner
=======================

gitlab-registry-cleaner 是一个批量清理 gitlab 镜像仓库的工具，支持多种方式匹配需要清理的镜像，方便集成到 gitlab CI 中。

# 使用方法


```
Usage of gitlab-registry-cleaner

Options:
  -T, --token string           Gitlab private token, environment: GITLAB_TOKEN
      --base-url string        Gitlab base url, environment: GITLAB_BASE_URL (default "https://gitlab.com/")
  -p, --project string         [REQUIRED]The ID or path of the project, environment: GITLAB_PROJECT
  -r, --registry stringArray   Registry repository path regex list, clean all repositories in project if registry not set
  -t, --tag stringArray        Image tag regex list
  -e, --exclude stringArray    Exclude image tag regex list
  -k, --keep-n int             Keeps N latest matching tagsRegex for each registry repositories (default 10)
  -o, --older-then string      Tags to delete that are older than the given time, written in human readable form 1h, 1d, 1m
  -n, --dry-run                Only print which images would be deleted
  -K, --insecure               Allow connections to SSL sites without certs
  -v, --verbose                Verbose output
  -V, --version                Print version and exit
  -h, --help                   Print help and exit
```

# Example

清理`foo/bar`工程中一个月以上除版本号以外的镜像，但是至少会保留最近的 5 个

```
export GITLAB_TOKEN=<gitlab-private-token>
gitlab-registry-cleaner -p foo/bar --older-then 1month --keep-n 5 --exclude '^v?[0-9.]+$'
```

在 CI 中使用请参考 [`.gitlab-ci.yml`](.gitlab-ci.yml)