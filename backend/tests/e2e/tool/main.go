// Command tool provides deterministic nmap and LibreOffice behavior for e2e tests.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	args := os.Args[1:]
	for _, arg := range args {
		if arg == "--convert-to" {
			runLibreOffice(args)
			return
		}
	}
	fmt.Print(`<?xml version="1.0" encoding="UTF-8"?>
<nmaprun>
  <host>
    <status state="up"/>
    <address addr="10.0.0.10" addrtype="ipv4"/>
    <address addr="02:00:00:00:00:10" addrtype="mac"/>
    <hostnames><hostname name="e2e-discovered"/></hostnames>
    <ports>
      <port protocol="tcp" portid="22">
        <state state="open"/>
        <service name="ssh" product="OpenSSH" version="9.0"/>
      </port>
    </ports>
  </host>
</nmaprun>`)
}

func runLibreOffice(args []string) {
	var outputDir, input string
	for index := 0; index < len(args); index++ {
		if args[index] == "--outdir" && index+1 < len(args) {
			outputDir = args[index+1]
			index++
			continue
		}
		if strings.HasSuffix(strings.ToLower(args[index]), ".docx") {
			input = args[index]
		}
	}
	if outputDir == "" || input == "" {
		fmt.Fprintln(os.Stderr, "missing --outdir or input DOCX")
		os.Exit(2)
	}
	name := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)) + ".pdf"
	if err := os.WriteFile(filepath.Join(outputDir, name), []byte("%PDF-1.4\n% e2e archive\n"), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
