# c9s - Containerlab on Kubernetes

`c9s` is a command-line tool designed to convert [Containerlab](https://containerlab.dev) topologies into [Clabernetes](https://c9s.run) Custom Resource Definitions (`c9s.run/v1alpha1`) and manage their deployment lifecycle directly on Kubernetes.

```
 ⣴⡾⠛⠛⠖ ⢸⣿      ⣾⣿⡀  ⢸⣿⠛⠛⣷⡄                               ⢸⡇               
⢸⣿     ⢸⣿     ⣸⡏⢹⣧  ⢸⣿⣀⣀⣾⠇ ⢠⣶⠟⠛⢷⣦ ⢸⣿⠛⠛⣷⡄ ⢸⣿⠛⠛⣷⣦ ⢠⣶⠟⠛⢷⣦ ⠘⠛⣿⡟⠛ ⢠⣶⠟⠛⢷⣦ ⣴⡟⠛⠛⢻⣦
⠘⣿⣄  ⡀ ⢸⣿    ⢠⣿⠷⠶⢿⡆ ⢸⣿⠉⠉⣷⡆ ⢸⣿⣤⣤⠾⠃ ⢸⣿     ⢸⣿  ⢸⣿ ⢸⣿⣤⣤⠾⠃   ⢸⡇  ⢸⣿⣤⣤⠾⠃ ⣈⡛⠛⠛⣿⡆
 ⠈⠙⠛⠛⠉ ⠘⠛⠛⠛⠛  ⠚⠃ ⠘⠛ ⠘⠛⠛⠛⠋   ⠈⠙⠛⠛⠉ ⠘⠛     ⠘⠛  ⠘⠛  ⠈⠙⠛⠛⠉   ⠘⠛   ⠈⠙⠛⠛⠉ ⠈⠛⠛⠛⠛⠁
```

## Features

- **Namespace Isolation:** Automatically encapsulates each lab in an isolated namespace named `lab-{name_of_lab}` stamped with privileged Pod Security Admission labels.
- **ConfigMap Synthesis:** Local `startup-config` files referenced by nodes are safely read and created as ConfigMaps in the lab's namespace.
- **File Mounting:** Injects `filesFromConfigMap` entries into the `Topology` CRD using the config's filename as the key.
- **Core Node & Link Support:** Supports nodes (`kind`, `image`, `startup-config`, `exec`) and link `endpoints`.
- **Familiar CLI Experience:** Uses `c9s deploy`, `c9s inspect`, and `c9s destroy`, auto-discovering `*.clab.yml` or `*.clab.yaml` in the current working directory, or accepting an explicit file with `-t / --topo`.
- **Containerlab Aesthetics:** Terminal colors, braille ASCII logo banner, structured timestamped logs, and tabular node status summaries.
- **Zero-Tolerance Safety Guards:**
  - Automatically defaults to `kind-try-c9s` context if present.
  - Prohibits mutating or deploying to the `default` or system namespaces.
  - Strict path traversal defenses preventing startup-config breakout outside the topology directory.

## Installation

```bash
# Build binary
go build -o bin/c9s ./cmd/c9s

# Install to user PATH
cp bin/c9s ~/.local/bin/c9s
```

## Usage

### 1. Version
```bash
c9s version
```

### 2. Deploy a Lab
```bash
# Deploy with auto-discovery in current directory
c9s deploy

# Deploy specifying topology file explicitly
c9s deploy -t examples/demo-lab/demo-lab.clab.yml
```

### 3. Inspect a Lab
```bash
# Inspect with auto-discovery in current directory
c9s inspect

# Inspect specifying topology file explicitly
c9s inspect -t examples/demo-lab/demo-lab.clab.yml

# Inspect specifying lab name directly
c9s inspect mytest

# Inspect all deployed labs
c9s inspect --all
```

### 4. Destroy a Lab
```bash
# Destroy with auto-discovery in current directory
c9s destroy

# Destroy specifying topology file explicitly
c9s destroy -t examples/demo-lab/demo-lab.clab.yml
```
