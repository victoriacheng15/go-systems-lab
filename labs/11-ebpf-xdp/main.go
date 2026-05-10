package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	labDir      = "labs/11-ebpf-xdp"
	sourcePath  = labDir + "/bpf/xdp_pass.c"
	objectPath  = labDir + "/build/xdp_pass.o"
	pinnedPath  = "/sys/fs/bpf/xdp_pass_lab"
	sysClassNet = "/sys/class/net"
)

type probeResult struct {
	Name   string
	Status string
	Detail string
}

type interfaceStatus struct {
	Name      string
	Index     int
	Hardware  string
	Flags     net.Flags
	OperState string
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "probe":
		if err := runProbe(); err != nil {
			fmt.Fprintf(os.Stderr, "probe: %v\n", err)
			os.Exit(1)
		}
	case "interfaces":
		if err := runInterfaces(); err != nil {
			fmt.Fprintf(os.Stderr, "interfaces: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := runStatus(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "status: %v\n", err)
			os.Exit(1)
		}
	case "commands":
		if err := runCommands(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "commands: %v\n", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage:")
	fmt.Println("  go run labs/11-ebpf-xdp/main.go probe")
	fmt.Println("  go run labs/11-ebpf-xdp/main.go interfaces")
	fmt.Println("  go run labs/11-ebpf-xdp/main.go status <interface>")
	fmt.Println("  go run labs/11-ebpf-xdp/main.go commands <interface>")
}

func runProbe() error {
	results := collectProbeResults()
	for _, result := range results {
		fmt.Printf("%-32s %-8s %s\n", result.Name, result.Status, result.Detail)
	}
	return nil
}

func runInterfaces() error {
	statuses, err := collectInterfaceStatuses()
	if err != nil {
		return err
	}
	for _, status := range statuses {
		printInterfaceStatus(status)
	}
	return nil
}

func runStatus(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("status requires exactly one interface name")
	}

	status, err := getInterfaceStatus(args[0])
	if err != nil {
		return err
	}
	printInterfaceStatus(status)
	return nil
}

func runCommands(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("commands requires exactly one interface name")
	}
	iface := args[0]
	if err := validateInterfaceName(iface); err != nil {
		return err
	}

	for _, command := range buildWorkflowCommands(iface) {
		fmt.Println(command)
	}
	return nil
}

func collectProbeResults() []probeResult {
	return []probeResult{
		checkPath("bpffs mount path", "/sys/fs/bpf"),
		checkPath("BPF syscall sysctl", "/proc/sys/kernel/unprivileged_bpf_disabled"),
		checkPath("BPF JIT sysctl", "/proc/sys/net/core/bpf_jit_enable"),
		checkCommand("clang compiler", "clang"),
		checkCommand("bpftool", "bpftool"),
		checkCommand("iproute2 ip", "ip"),
		checkPath("XDP source", sourcePath),
	}
}

func checkPath(name, path string) probeResult {
	resolved, err := resolveProbePath(path)
	if err != nil {
		return probeResult{Name: name, Status: "missing", Detail: path}
	}

	detail := resolved
	if value, err := readSmallFile(resolved); err == nil && value != "" {
		detail = fmt.Sprintf("%s=%s", resolved, value)
	}
	return probeResult{Name: name, Status: "ok", Detail: detail}
}

func resolveProbePath(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if filepath.IsAbs(path) {
		return "", os.ErrNotExist
	}

	fromPackageDir := filepath.Join("..", "..", path)
	if _, err := os.Stat(fromPackageDir); err == nil {
		return fromPackageDir, nil
	}
	return "", os.ErrNotExist
}

func checkCommand(name, command string) probeResult {
	path, err := exec.LookPath(command)
	if err != nil {
		return probeResult{Name: name, Status: "missing", Detail: command}
	}
	return probeResult{Name: name, Status: "ok", Detail: path}
}

func readSmallFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() || info.Size() > 4096 {
		return "", nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func collectInterfaceStatuses() ([]interfaceStatus, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	statuses := make([]interfaceStatus, 0, len(interfaces))
	for _, iface := range interfaces {
		statuses = append(statuses, statusFromInterface(iface))
	}

	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses, nil
}

func getInterfaceStatus(name string) (interfaceStatus, error) {
	if err := validateInterfaceName(name); err != nil {
		return interfaceStatus{}, err
	}

	iface, err := net.InterfaceByName(name)
	if err != nil {
		return interfaceStatus{}, err
	}
	return statusFromInterface(*iface), nil
}

func statusFromInterface(iface net.Interface) interfaceStatus {
	return interfaceStatus{
		Name:      iface.Name,
		Index:     iface.Index,
		Hardware:  iface.HardwareAddr.String(),
		Flags:     iface.Flags,
		OperState: readOperState(iface.Name),
	}
}

func readOperState(name string) string {
	if err := validateInterfaceName(name); err != nil {
		return "unknown"
	}

	content, err := os.ReadFile(filepath.Join(sysClassNet, name, "operstate"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(content))
}

func validateInterfaceName(name string) error {
	if name == "" {
		return fmt.Errorf("interface name cannot be empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("interface name %q cannot contain path separators", name)
	}
	return nil
}

func buildWorkflowCommands(iface string) []string {
	return []string{
		"mkdir -p labs/11-ebpf-xdp/build",
		fmt.Sprintf("clang -O2 -target bpf -c %s -o %s", sourcePath, objectPath),
		fmt.Sprintf("llvm-objdump -h %s", objectPath),
		fmt.Sprintf("sudo bpftool prog load %s %s type xdp", objectPath, pinnedPath),
		fmt.Sprintf("sudo bpftool net attach xdpgeneric pinned %s dev %s overwrite", pinnedPath, iface),
		fmt.Sprintf("sudo bpftool net show dev %s", iface),
		fmt.Sprintf("sudo bpftool net detach xdpgeneric dev %s", iface),
	}
}

func printInterfaceStatus(status interfaceStatus) {
	hardware := status.Hardware
	if hardware == "" {
		hardware = "-"
	}
	fmt.Printf("%-16s index=%-4d state=%-10s flags=%-18s mac=%s\n", status.Name, status.Index, status.OperState, status.Flags.String(), hardware)
}
