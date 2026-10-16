package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/TrebuchetDynamics/go-browser-harness/internal/buildinfo"
	"github.com/TrebuchetDynamics/go-browser-harness/pkg/harness"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	actionJSON := flag.String("action-json", "", "execute one Gormes browser action JSON envelope")
	doctor := flag.Bool("doctor", false, "probe environment health and print a diagnostic table")
	flag.Parse()

	if *doctor {
		env := DoctorEnv{
			ChromeRemoteDebuggingURL: os.Getenv("CHROME_REMOTE_DEBUGGING_URL"),
			BrowserUseAPIKeyPresent:  os.Getenv("BROWSER_USE_API_KEY") != "",
			HTTPClient:               http.DefaultClient,
		}
		os.Exit(runDoctor(context.Background(), env, os.Stdout))
	}

	if *showVersion {
		if err := writeLine(os.Stdout, buildinfo.Version); err != nil {
			log.Fatalf("write version: %v", err)
		}

		return
	}
	if *actionJSON != "" {
		req, err := harness.ParseActionJSON(*actionJSON)
		if err != nil {
			printActionResult(harness.ActionResult{
				SchemaVersion: harness.ActionSchemaVersion,
				Evidence:      harness.EvidenceInvalidAction,
				Message:       err.Error(),
			})
			os.Exit(2)
		}
		backend := resolveBackend(context.Background())
		result, err := harness.RunAction(context.Background(), req, backend)
		printActionResult(result)
		if err != nil {
			os.Exit(1)
		}
		return
	}

	if err := writeLine(os.Stdout, "go-browser-harness: thin chromedp harness scaffold"); err != nil {
		log.Fatalf("write status: %v", err)
	}
}

// resolveBackend picks a live chromedp backend when CHROME_REMOTE_DEBUGGING_URL
// is set, or falls back to UnavailableBackend which returns typed evidence
// without launching Chrome.
func resolveBackend(ctx context.Context) harness.Backend {
	backend, err := harness.NewChromedpBackendFromEnv(ctx)
	if err != nil || backend == nil {
		return harness.UnavailableBackend{Reason: "CDP backend not configured; set CHROME_REMOTE_DEBUGGING_URL"}
	}
	return backend
}

func writeLine(w io.Writer, line string) error {
	_, err := fmt.Fprintln(w, line)
	return err
}

func printActionResult(result harness.ActionResult) {
	raw, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("encode action result: %v", err)
	}
	if err := writeLine(os.Stdout, string(raw)); err != nil {
		log.Fatalf("write action result: %v", err)
	}
}
