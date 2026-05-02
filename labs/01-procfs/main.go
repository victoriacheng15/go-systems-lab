package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type coreStats struct {
	user   uint64
	system uint64
	idle   uint64
	total  uint64
}

type netStats struct {
	rxBytes uint64
	txBytes uint64
}

func main() {
	cpuFlag := flag.Bool("cpu", false, "Display CPU load averages")
	coresFlag := flag.Bool("cores", false, "Display individual CPU core stats")
	memFlag := flag.Bool("mem", false, "Display Memory information")
	netFlag := flag.Bool("net", false, "Display Network interface stats")
	powerFlag := flag.Bool("power", false, "Display CPU power consumption (RAPL)")
	liveFlag := flag.Bool("live", false, "Update output live (interactive mode)")
	flag.Parse()

	showAll := !*cpuFlag && !*coresFlag && !*memFlag && !*netFlag && !*powerFlag
	sections := []string{"cpu", "cores", "net", "power", "mem"}

	for {
		if *liveFlag {
			// ANSI Escape: Clear screen and move cursor to top-left
			fmt.Print("\033[H\033[2J")
			fmt.Printf("--- Go Systems Lab: Live Telemetry (%s) ---\n\n", time.Now().Format(time.RFC1123))
		}

		first := true
		for _, s := range sections {
			var err error
			var run bool

			switch s {
			case "cpu":
				if showAll || *cpuFlag {
					err = printCPULoad()
					run = true
				}
			case "cores":
				if showAll || *coresFlag {
					err = printCPUCores()
					run = true
				}
			case "net":
				if showAll || *netFlag {
					err = printNetDev()
					run = true
				}
			case "power":
				if showAll || *powerFlag {
					err = printPowerUsage()
					run = true
				}
			case "mem":
				if showAll || *memFlag {
					err = printMemInfo()
					run = true
				}
			}

			if run {
				if !first {
					fmt.Println()
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
				first = false
			}
		}

		if !*liveFlag {
			break
		}
		// If live, wait a bit before the next iteration
		time.Sleep(1 * time.Second)
	}
}

// --- Parsing Logic (Decoupled from OS for testing) ---

func parseCPULoad(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 {
			return fmt.Sprintf("1 min:  %s procs\n5 min:  %s procs\n15 min: %s procs", fields[0], fields[1], fields[2]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("unexpected loadavg format")
}

func parseCPUSamples(r io.Reader) (map[string]coreStats, error) {
	samples := make(map[string]coreStats)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		name := fields[0]
		if name == "cpu" || !unicode.IsDigit(rune(name[3])) {
			continue
		}

		var total uint64
		for i := 1; i < len(fields); i++ {
			val, _ := strconv.ParseUint(fields[i], 10, 64)
			total += val
		}

		user, _ := strconv.ParseUint(fields[1], 10, 64)
		system, _ := strconv.ParseUint(fields[3], 10, 64)
		idle, _ := strconv.ParseUint(fields[4], 10, 64)

		samples[name] = coreStats{user: user, system: system, idle: idle, total: total}
	}
	return samples, scanner.Err()
}

func parseNetSamples(r io.Reader) (map[string]netStats, error) {
	samples := make(map[string]netStats)
	scanner := bufio.NewScanner(r)
	scanner.Scan() // skip header 1
	scanner.Scan() // skip header 2

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 9 {
			continue
		}

		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		samples[iface] = netStats{rxBytes: rx, txBytes: tx}
	}
	return samples, scanner.Err()
}

func parseMemInfo(r io.Reader, limit int) ([]string, error) {
	var output []string
	scanner := bufio.NewScanner(r)
	count := 0
	for scanner.Scan() && count < limit {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			kbVal, _ := strconv.ParseFloat(fields[1], 64)
			mbVal := kbVal / 1024.0
			output = append(output, fmt.Sprintf("%-30s (%8.1f MB)", line, mbVal))
		} else {
			output = append(output, line)
		}
		count++
	}
	return output, scanner.Err()
}

// --- I/O Wrappers ---

func printCPULoad() error {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return err
	}
	defer f.Close()

	out, err := parseCPULoad(f)
	if err != nil {
		return err
	}
	fmt.Println("--- CPU Load Averages ---")
	fmt.Println(out)
	return nil
}

func getCPUSamples() (map[string]coreStats, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseCPUSamples(f)
}

func printCPUCores() error {
	s1, err := getCPUSamples()
	if err != nil {
		return err
	}
	// Note: Shortened sampling time slightly for live mode feel
	time.Sleep(250 * time.Millisecond)
	s2, err := getCPUSamples()
	if err != nil {
		return err
	}

	fmt.Printf("%-8s | %-20s | %-20s | %-20s\n", "Core", "User", "System", "Idle")
	fmt.Println(strings.Repeat("-", 75))

	for i := 0; i < len(s2); i++ {
		name := fmt.Sprintf("cpu%d", i)
		v1, ok1 := s1[name]
		v2, ok2 := s2[name]
		if !ok1 || !ok2 {
			continue
		}
		totalDelta := float64(v2.total - v1.total)
		if totalDelta == 0 {
			totalDelta = 1
		}
		userPct := (float64(v2.user-v1.user) / totalDelta) * 100
		sysPct := (float64(v2.system-v1.system) / totalDelta) * 100
		idlePct := (float64(v2.idle-v1.idle) / totalDelta) * 100

		fmt.Printf("%-8s | %10d (%5.1f%%) | %10d (%5.1f%%) | %10d (%5.1f%%)\n",
			name, v2.user, userPct, v2.system, sysPct, v2.idle, idlePct)
	}
	return nil
}

func getNetSamples() (map[string]netStats, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseNetSamples(f)
}

func printNetDev() error {
	s1, err := getNetSamples()
	if err != nil {
		return err
	}
	time.Sleep(250 * time.Millisecond)
	s2, err := getNetSamples()
	if err != nil {
		return err
	}

	maxIfaceLen := 9
	for iface := range s2 {
		if len(iface) > maxIfaceLen {
			maxIfaceLen = len(iface)
		}
	}

	fmt.Printf("%-*s | %-25s | %-25s\n", maxIfaceLen, "Interface", "Received (RX)", "Transmitted (TX)")
	fmt.Println(strings.Repeat("-", maxIfaceLen+56))

	for iface, v2 := range s2 {
		v1, ok := s1[iface]
		if !ok {
			continue
		}
		// delta / 0.25s = delta * 4
		rxRate := float64(v2.rxBytes-v1.rxBytes) * 4.0
		txRate := float64(v2.txBytes-v1.txBytes) * 4.0
		fmt.Printf("%-*s | %10d bytes (%8.1f KB/s) | %10d bytes (%8.1f KB/s)\n",
			maxIfaceLen, iface, v2.rxBytes, rxRate/1024.0, v2.txBytes, txRate/1024.0)
	}
	return nil
}

func readEnergyUJ() (uint64, error) {
	data, err := os.ReadFile("/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj")
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func printPowerUsage() error {
	uj1, err := readEnergyUJ()
	if err != nil {
		return fmt.Errorf("Intel RAPL restricted or not available: %v", err)
	}
	time.Sleep(250 * time.Millisecond)
	uj2, err := readEnergyUJ()
	if err != nil {
		return err
	}
	// microjoules to watts: (uj_delta / 0.25s) / 1,000,000 = delta * 4 / 1,000,000
	watts := (float64(uj2-uj1) * 4.0) / 1000000.0
	fmt.Printf("Cumulative Energy: %d microjoules\n", uj2)
	fmt.Printf("Current Draw:      %.2f Watts\n", watts)
	return nil
}

func printMemInfo() error {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return err
	}
	defer f.Close()

	out, err := parseMemInfo(f, 5)
	if err != nil {
		return err
	}
	fmt.Println("--- Memory Information ---")
	for _, line := range out {
		fmt.Println(line)
	}
	return nil
}
