package main

// This file is the CLI entry point for offline validation. It exists so
// firmware can be verified without starting the Electron app or the stdio
// protocol, and so a long validation can report progress and a final verdict
// on stdout that survives a terminal disconnect.

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

// runValidateCLI implements `autoroot-validate validate`.
func runValidateCLI(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	archive := fs.String("archive", "", "path to the firmware .zip")
	model := fs.String("model", "", "expected model, e.g. SM-A065F")
	csc := fs.String("csc", "", "expected sales code, e.g. XID")
	binary := fs.String("binary", "", "device bootloader binary revision, e.g. 4")
	extractDir := fs.String("extract", "", "folder to extract into (defaults to <firmware root>/extracted)")
	jsonOut := fs.Bool("json", false, "emit the report as JSON")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *archive == "" || *model == "" {
		fmt.Fprintln(os.Stderr, "usage: validate -archive <file> -model <SM-XXXX> [-csc XID] [-binary 4] [-extract dir] [-json]")
		return 2
	}

	dir := *extractDir
	if dir == "" {
		dir = ""
	}
	if dir == "" {
		fmt.Fprintf(os.Stderr, "Firmware root: %s\n", resolveFirmwareRoot())
		dir = deviceFirmwareDir(*model) + string(os.PathSeparator) + "extracted"
	}

	started := time.Now()
	fmt.Fprintf(os.Stderr, "Validating %s\n", *archive)
	fmt.Fprintf(os.Stderr, "Expecting model %s, CSC %s, binary %s\n", *model, *csc, *binary)
	fmt.Fprintf(os.Stderr, "Extracting to %s\n\n", dir)

	v := ValidateFirmwareArchive(*archive, FirmwareProfile{
		Model:        *model,
		CSC:          *csc,
		DeviceBinary: *binary,
	}, dir)

	elapsed := time.Since(started).Seconds()

	if *jsonOut {
		out := map[string]interface{}{
			"ok":        v.OK,
			"errors":    v.Errors,
			"warnings":  v.Warnings,
			"modelCode": v.ModelCode,
			"binary":    v.PkgBinary,
			"csc":       v.CSCInPackage,
			"seconds":   elapsed,
			"slots":     map[string]interface{}{},
		}
		for slot, sf := range v.Sizes {
			out["slots"].(map[string]interface{})[string(slot)] = map[string]interface{}{
				"file": sf.FileName,
				"path": sf.Path,
				"size": sf.Size,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		if v.OK {
			return 0
		}
		return 1
	}

	fmt.Printf("Model code:   %s\n", v.ModelCode)
	fmt.Printf("Package bin:  %s\n", v.PkgBinary)
	fmt.Printf("CSC in pkg:   %s\n", v.CSCInPackage)
	fmt.Printf("\nSlots:\n")
	for _, slot := range RequiredSlots {
		sf, ok := v.Sizes[slot]
		if !ok {
			fmt.Printf("  %-4s MISSING\n", slot)
			continue
		}
		fmt.Printf("  %-4s %s  (%s)\n", slot, sf.FileName, formatSize(sf.Size))
	}
	if len(v.Warnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		for _, w := range v.Warnings {
			fmt.Printf("  ! %s\n", w)
		}
	}
	if len(v.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, e := range v.Errors {
			fmt.Printf("  x %s\n", e)
		}
	}
	fmt.Printf("\nTook %.0fs\n", elapsed)
	if v.OK {
		fmt.Println("RESULT: VALID")
		return 0
	}
	fmt.Println("RESULT: INVALID")
	return 1
}
