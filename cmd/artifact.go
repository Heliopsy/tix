package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// artifactKinds are the kinds an artifact may take.
var artifactKinds = []string{
	string(core.ArtifactResult), string(core.ArtifactLog),
	string(core.ArtifactFile), string(core.ArtifactMetric),
}

// newArtifactCmd builds the artifact command group.
func newArtifactCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "artifact",
		Short:   "Attach and read task artifacts",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(artifactPutCmd(g), artifactLsCmd(g))
	return cmd
}

func artifactPutCmd(g *globals) *cobra.Command {
	var kind, name, contentType, payload, file string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "put REF",
		Short: "Attach structured output to a task",
		Long: "Attach an artifact to a task.\n" +
			"The payload is a JSON object given with --payload, or - to read standard input.\n" +
			"--file attaches the bytes of a file as the artifact blob, or - to read standard input.\n\n" +
			fmt.Sprintf("Exit codes: %d invalid input, %d unknown reference, %d permission denied.",
				core.KindInvalid.ExitCode(), core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix artifact put default-1 --payload '{\"passed\":true}'\n" +
			"  tix artifact put default-1 --kind log --file build.log --content-type text/plain\n" +
			"  cat report.json | tix artifact put default-1 --name report --payload -",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			in, err := artifactInput(cmd, artifactFlags{
				kind: kind, name: name, contentType: contentType,
				payload: payload, file: file,
			})
			if err != nil {
				return err
			}
			if err := in.Validate(); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.dryBulk(cmd, conn, ctx, []core.TaskRef{ref}, "artifact.put",
					map[string]any{"kind": string(in.Kind), "name": in.Name})
			}
			artifact, err := conn.Service.PutArtifact(ctx, ref, in)
			if err != nil {
				return err
			}
			return g.render(cmd, artifact)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", string(core.ArtifactResult), "artifact kind: "+strings.Join(artifactKinds, "|"))
	f.StringVar(&name, "name", "", "name that identifies the artifact on the task")
	f.StringVar(&contentType, "content-type", "", "media type of the attached blob")
	f.StringVar(&payload, "payload", "", "json object to record, or - to read standard input")
	f.StringVar(&file, "file", "", "file whose bytes are attached as the blob, or - to read standard input")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be attached without writing")
	_ = cmd.RegisterFlagCompletionFunc("kind", fixedCompletion(artifactKinds))
	return cmd
}

func artifactLsCmd(g *globals) *cobra.Command {
	return &cobra.Command{
		Use:     "ls REF",
		Aliases: []string{"list"},
		Short:   "List the artifacts attached to a task",
		Long: "List a task's artifacts, oldest first.\n\n" +
			fmt.Sprintf("Exit codes: %d unknown reference, %d permission denied.",
				core.KindNotFound.ExitCode(), core.KindForbidden.ExitCode()),
		Example: "  tix artifact ls default-1 -o json",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			artifacts, err := conn.Service.ListArtifacts(ctx, ref)
			if err != nil {
				return err
			}
			out := newList[core.Artifact](g, cmd)
			for _, a := range artifacts {
				if err := out.Write(a); err != nil {
					return out.fail(err)
				}
			}
			return out.Close()
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
}

// artifactFlags are the values artifact put collects from the command line.
type artifactFlags struct {
	kind        string
	name        string
	contentType string
	payload     string
	file        string
}

// artifactInput builds the input the service takes, reading the payload and
// the blob from the sources the flags name.
func artifactInput(cmd *cobra.Command, f artifactFlags) (core.ArtifactInput, error) {
	in := core.ArtifactInput{
		Kind:        core.ArtifactKind(strings.TrimSpace(f.kind)),
		Name:        f.name,
		ContentType: f.contentType,
	}
	if f.payload == StdinMarker && f.file == StdinMarker {
		return in, core.Invalid("only one of --payload and --file may read standard input")
	}
	if f.payload != "" {
		raw, err := body(cmd, f.payload)
		if err != nil {
			return in, err
		}
		payload, err := decodePayload(raw)
		if err != nil {
			return in, err
		}
		in.Payload = payload
	}
	if f.file != "" {
		blob, err := readBlob(cmd, f.file)
		if err != nil {
			return in, err
		}
		in.Blob = blob
	}
	return in, nil
}

// decodePayload reads a JSON object, treating an empty document as none.
func decodePayload(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, core.Invalid("artifact payload must be a json object: %v", err)
	}
	return out, nil
}

// readBlob returns the bytes of a file, or of standard input for "-".
func readBlob(cmd *cobra.Command, path string) ([]byte, error) {
	if path == StdinMarker {
		blob, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, core.Internal("reading standard input").Wrap(err)
		}
		return blob, nil
	}
	blob, err := os.ReadFile(path) // #nosec G304 -- the path is chosen by the user
	if err != nil {
		return nil, core.Invalid("reading artifact file %q: %v", path, err)
	}
	return blob, nil
}
