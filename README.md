# mini-cni

**A custom Kubernetes Container Network Interface (CNI) plugin built from scratch.**

`mini-cni` is an educational project exploring how Kubernetes Pod networking works under the hood. The goal is to implement a minimal CNI plugin and progressively build networking capabilities using Linux network namespaces, veth pairs, Linux bridges, IP Address Management (IPAM), and routing.

Inspired by the article: [Create a CNI Plugin from Scratch — Simone Rodigari](https://www.srodi.com/posts/create-a-cni-plugin-from-scratch/).

## 🎯 Project Goals

- Understand the Kubernetes CNI specification and plugin lifecycle.
- Learn how container runtimes configure Pod networking.
- Implement CNI operations such as `ADD`, `DEL`, `CHECK`, `STATUS`, and `VERSION`.
- Create and manage Linux network namespaces and virtual Ethernet pairs.
- Implement IP Address Management (IPAM).
- Enable Pod-to-Pod communication on the same node.
- Explore cross-node Pod networking and routing.
- Compare Bash and Go implementations of a CNI plugin.

## 🏗️ Architecture

The initial implementation uses a Linux bridge to connect Pods on the same node.

```text
                     Kubernetes Node
+--------------------------------------------------+
|                                                  |
|  Pod A Network Namespace     Pod B Network NS    |
|  +--------------------+      +----------------+  |
|  | eth0               |      | eth0           |  |
|  | 10.10.0.2/24       |      | 10.10.0.3/24   |  |
|  +--------+-----------+      +--------+-------+  |
|           |                           |          |
|        veth pair                   veth pair     |
|           |                           |          |
|  +--------+---------------------------+-------+  |
|  |                cni0 Bridge                 |  |
|  +--------------------------------------------+  |
|                                                  |
+--------------------------------------------------+
```

The CNI plugin connects a Pod's network namespace to the host networking infrastructure. IPAM allocates an IP address, and the host configures the necessary bridge and routes.

## 🚀 Learning Roadmap

### Phase 1 — Basic CNI

- Create a Pod network namespace.
- Configure a veth pair.
- Move one end into the Pod namespace as `eth0`.
- Assign a fixed IP address.
- Implement the basic CNI `ADD` and `DEL` operations.

### Phase 2 — IP Address Management

- Allocate IP addresses dynamically.
- Prevent duplicate IP allocation.
- Track allocated addresses.
- Release addresses when Pods are removed.

### Phase 3 — Same-Node Pod Networking

- Create a Linux bridge named `cni0`.
- Attach host-side veth interfaces to the bridge.
- Configure Pod routes.
- Test connectivity between Pods on the same node.

### Phase 4 — Cross-Node Networking

- Configure Pod CIDRs for multiple nodes.
- Set up host routes for remote Pod networks.
- Enable Pod-to-Pod connectivity across nodes.
- Investigate forwarding, routing, and reverse-path filtering.

### Phase 5 — CNI Lifecycle and Reliability

- Implement `CHECK`, `STATUS`, and `VERSION` where supported by the target CNI specification.
- Handle repeated calls and cleanup safely.
- Validate configuration and report errors.
- Test failure scenarios and resource cleanup.

## 🛠️ Technology Stack

- **Kubernetes networking:** CNI specification
- **Languages:** Go and Bash
- **Linux networking:** Network namespaces, veth pairs, bridges, routes, and IP forwarding
- **IPAM:** Custom IP address allocation
- **Testing:** Network connectivity and CNI lifecycle tests

The Go implementation requires Go 1.24 or newer, as specified in the tutorial.

## ⚙️ Platform Requirements

The networking components depend on Linux kernel networking features and privileges.

- Linux host or Linux virtual machine
- Root privileges or equivalent networking capabilities
- Linux network namespace and `iproute2` utilities
- CNI-compatible runtime or a controlled test environment
- Go 1.24+ for the Go implementation

**macOS note:** The article's host tooling may run on macOS, but Linux bridge, network namespace, and veth operations require a Linux kernel. On macOS, use a Linux VM for networking experiments.

## 📚 Reference

This project follows the concepts described in:

**Create a CNI Plugin from Scratch**  
Author: Simone Rodigari  
Article: https://www.srodi.com/posts/create-a-cni-plugin-from-scratch/

The article provides the foundation for implementing a bridge-based CNI plugin and progressively adding IPAM and routing.

## ⚠️ Scope and Limitations

This is a learning project, not a production-ready Kubernetes networking solution.

The initial implementation does not provide NetworkPolicy enforcement, encryption, IPv6, observability, or automatic route distribution. Additional work is required before using it in a real cluster.

## 🤝 Contributions

Suggestions, experiments, issues, and improvements are welcome. The main objective is to understand Kubernetes networking by implementing its building blocks from the ground up.

## 📄 License

Choose an appropriate open-source license before distributing the project. Until a license is added, the repository should not be assumed to grant permission to reuse its code.
