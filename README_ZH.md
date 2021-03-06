gitlab-registry-cleaner
=======================

gitlab-registry-cleaner 是一个批量清理 gitlab 镜像仓库的工具，支持多种方式匹配需要清理的镜像，方便集成到 gitlab CI 中。

# 注意

目前 Gitlab 不支持直接使用 `CI_JOB_TOKEN` 删除镜像，所以需要使用有权限的用户来创建 private token，相对不便，参考

[#29566- Allow API project access with ci_job_token for internal project or public project with member only access to repository or private project](https://gitlab.com/gitlab-org/gitlab-ce/issues/29566)
[#41084 - create ci-extended-job-token in the ci-job-info](https://gitlab.com/gitlab-org/gitlab-ce/issues/41084)

# 使用方法

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

清理`foo/bar`工程中一个月以上除版本号以外的镜像，但是至少会保留最近的 5 个

```
export GITLAB_TOKEN=<gitlab-private-token>
gitlab-registry-cleaner -p foo/bar --older-then 1month --keep-n 5 --exclude '^v?[0-9.]+$'
```

在 CI 中使用请参考 [`.gitlab-ci.yml`](.gitlab-ci.yml)