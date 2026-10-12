package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kuasar-sandbox/accelerator/pkg/image"
)

func TestConversionPolicyFailsBeforeWork(t *testing.T) {
	for _, args := range [][]string{{"export"}, {"referer", "lookup"}, {"referer", "put"}} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			// The config is deliberately absent: policy mismatch must be
			// diagnosed before loading configuration or touching a registry.
			call := append([]string{"-test.run=^TestConversionPolicyProcess$", "--", "--conversion-helper"}, args...)
			call = append(call, "--conversion-semantics", "old-generator-policy", "--config", filepath.Join(t.TempDir(), "missing.yaml"))
			cmd := exec.Command(exe, call...)
			out, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(out, []byte(`conversion semantics "old-generator-policy" required`)) || !bytes.Contains(out, []byte(image.ConversionSemantics)) {
				t.Fatalf("policy mismatch did not fail before work: %s %v", out, err)
			}
		})
	}
	for _, policy := range []string{"", image.ConversionSemantics} {
		if err := checkConversionSemantics(policy); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConversionPolicyProcess(t *testing.T) {
	i := slices.Index(os.Args, "--conversion-helper")
	if i < 0 {
		return
	}
	os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
	main()
	t.Fatal("incompatible generator accepted")
}
