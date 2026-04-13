# Kaniko Build and Push Action

A GitHub Action for building and pushing multi-platform container images using Kaniko on self-hosted runners with GitHub ARC.

## Features

- Build container images without Docker daemon
- Multi-platform support
- Layer caching support
- Secure credential handling
- Works with GitHub ARC (Actions Runner Controller)

## Usage

### Basic Example

```yaml
- name: Build and push with Kaniko
  uses: ./.github/actions/kaniko-build
  with:
    registry: ghcr.io
    image-name: my-org/my-image
    tags: |
      ghcr.io/my-org/my-image:latest
      ghcr.io/my-org/my-image:v1.0.0
    platform: linux/amd64
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

### Multi-Platform Build

```yaml
strategy:
  matrix:
    platform:
      - os: linux
        arch: amd64
      - os: linux
        arch: arm64

steps:
  - name: Build and push
    uses: ./.github/actions/kaniko-build
    with:
      registry: ghcr.io
      image-name: my-org/my-image
      tags: ${{ needs.prepare.outputs.tags }}
      platform: ${{ matrix.platform.os }}/${{ matrix.platform.arch }}
      tag-suffix: ${{ matrix.platform.os }}-${{ matrix.platform.arch }}
      registry-username: ${{ github.actor }}
      registry-password: ${{ secrets.GITHUB_TOKEN }}
```

### With Build Arguments

```yaml
- name: Build and push
  uses: ./.github/actions/kaniko-build
  with:
    registry: ghcr.io
    image-name: my-org/my-image
    tags: ghcr.io/my-org/my-image:latest
    platform: linux/amd64
    build-args: |
      VERSION=1.0.0
      COMMIT=${{ github.sha }}
      DATE=${{ github.event.head_commit.timestamp }}
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

### With Caching

```yaml
- name: Build and push
  uses: ./.github/actions/kaniko-build
  with:
    registry: ghcr.io
    image-name: my-org/my-image
    tags: ghcr.io/my-org/my-image:latest
    platform: linux/amd64
    cache: true
    cache-repo: ghcr.io/my-org/my-image/cache
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `context` | Build context path | No | `.` |
| `dockerfile` | Path to Dockerfile | No | `Dockerfile` |
| `registry` | Container registry URL | Yes | - |
| `image-name` | Image name (without registry) | Yes | - |
| `tags` | Image tags (newline separated) | Yes | - |
| `platform` | Target platform (e.g., linux/amd64) | Yes | - |
| `tag-suffix` | Suffix to append to each tag | No | `` |
| `build-args` | Build arguments (newline separated KEY=VALUE pairs) | No | `` |
| `cache` | Enable layer caching | No | `true` |
| `cache-repo` | Cache repository URL | No | `` |
| `push` | Push image to registry | No | `true` |
| `kaniko-version` | Kaniko executor version | No | `v1.23.0` |
| `kaniko-image` | Kaniko executor image | No | `gcr.io/kaniko-project/executor` |
| `registry-username` | Registry username | Yes | - |
| `registry-password` | Registry password | Yes | - |

## Outputs

| Output | Description |
|--------|-------------|
| `digest` | Image digest |

## Requirements

- Self-hosted runner with Kaniko support (e.g., GitHub ARC with Kaniko)
- Access to container registry
- Appropriate permissions to push images

## License

Same as parent repository
