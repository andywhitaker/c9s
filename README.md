# c9s

`c9s` is a command-line tool for deploying and managing [Containerlab](https://containerlab.dev) topologies on Kubernetes using [Clabernetes](https://c9s.run).


## Installation

Download the standalone binary for your platform directly from the [Releases](https://github.com/andywhitaker/c9s/releases) page:

```bash
# Example for Linux (x86_64)
curl -sLO https://github.com/andywhitaker/c9s/releases/latest/download/c9s-linux-amd64
chmod +x c9s-linux-amd64
sudo mv c9s-linux-amd64 /usr/local/bin/c9s
```

Or install via Go:

```bash
go install github.com/andywhitaker/c9s/cmd/c9s@latest
```

Or build from source:

```bash
git clone https://github.com/andywhitaker/c9s.git
cd c9s
go build -o c9s ./cmd/c9s
```

## Commands

### `c9s deploy`
Deploys a lab topology to Kubernetes.

```bash
# Auto-discover topology file in current directory
c9s deploy

# Specify topology file explicitly
c9s deploy -t topo.clab.yml
```

### `c9s inspect`
Displays a summary table of deployed nodes, states, and assigned IP addresses (both external LoadBalancer and internal management IPs).

```bash
# Inspect lab in current directory
c9s inspect

# Inspect a specific lab by name
c9s inspect mytest

# Inspect all deployed labs
c9s inspect --all
```

### `c9s connect`
Connects directly to a lab node. By default, network OS nodes (SR Linux, Arista, Cisco, etc.) connect via SSH, while Linux nodes connect via `kubectl exec`.

```bash
# Connect using default method (SSH for NOS, exec for Linux)
c9s connect srl1

# Specify username
c9s connect admin@srl1
c9s connect srl1 -u admin

# Force connection method
c9s connect srl1 --exec
c9s connect client1 --ssh

# Run a one-off command
c9s connect client1 -- ps aux
```

### `c9s destroy`
Tears down a deployed lab and removes its Kubernetes namespace.

```bash
# Destroy lab in current directory
c9s destroy

# Destroy by topology file
c9s destroy -t topo.clab.yml

# Destroy by lab name
c9s destroy mytest
```

### `c9s version`
Displays CLI version and build details.

```bash
c9s version
```

## Global Flags

- `-t, --topo <file>`: Path to topology definition file
- `--context <name>`: Kubernetes context to use (defaults to `kind-try-c9s` if present)
- `--kubeconfig <path>`: Path to kubeconfig file
- `--no-color`: Disable colored terminal output

## License

Apache-2.0
