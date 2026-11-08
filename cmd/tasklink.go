// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/tasklink"
	"github.com/spf13/cobra"
)

// taskLink is what `task link` prints in a structured format: every form the
// browser's Share menu copies.
type taskLink struct {
	Ref      string `json:"ref" yaml:"ref"`
	Title    string `json:"title" yaml:"title"`
	URL      string `json:"url" yaml:"url"`
	Markdown string `json:"markdown" yaml:"markdown"`
	Text     string `json:"text" yaml:"text"`
}

// taskLinkForms are the values --as takes.
var taskLinkForms = []string{"url", "markdown", "text"}

// taskLinkCmd builds `task link`, the command line's share: a task's address
// in the browser interface. It is formatting over GetTask rather than an
// operation of its own, because only the client knows the address it reaches
// the server at.
func taskLinkCmd(g *globals) *cobra.Command {
	var base, as string
	cmd := &cobra.Command{
		Use:   "link REF",
		Short: "Print a task's address in the browser interface",
		Long: "Print the absolute address of a task's screen in the browser interface, the link the " +
			"task screen's Share menu copies. The address is the server this context talks to, or " +
			"--base. A local database has no web address, so it needs --base naming the address " +
			"tix serve is reached at.\n\n" +
			"--as markdown prints a Markdown link and --as text the reference and title. -o json or " +
			"-o yaml prints all three. The link names no tenant: on a shared host the tenant comes from " +
			"the session of whoever opens it.\n\n" +
			"Exit codes: 2 invalid input or no address to build on, 3 unknown reference, 5 permission denied.",
		Example: "  tix task link infra-12\n  tix task link infra-12 --as markdown\n" +
			"  tix task link infra-12 --base https://tix.example.com -o json",
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			if !slices.Contains(taskLinkForms, as) {
				return core.Invalid("--as takes %s, not %q", strings.Join(taskLinkForms, ", "), as)
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			address, err := webBase(base, conn.Info.Target)
			if err != nil {
				return err
			}
			task, err := conn.Service.GetTask(ctx, ref)
			if err != nil {
				return err
			}
			link := newTaskLink(address, *task)
			if g.formatName() != output.FormatTable {
				return g.render(cmd, link)
			}
			line := map[string]string{"url": link.URL, "markdown": link.Markdown, "text": link.Text}[as]
			_, err = fmt.Fprintln(cmd.OutOrStdout(), line)
			return err
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().StringVar(&base, "base", "", "address of the browser interface, such as https://tix.example.com")
	cmd.Flags().StringVar(&as, "as", "url", "what to print: url, markdown or text")
	return cmd
}

// newTaskLink builds every form of the link to t under base.
func newTaskLink(base string, t core.Task) taskLink {
	ref := output.ShowRef(t.Ref)
	address := tasklink.URL(base, t.Ref)
	return taskLink{Ref: ref, Title: t.Title, URL: address,
		Markdown: tasklink.Markdown(ref, t.Title, address), Text: tasklink.Text(ref, t.Title)}
}

// webBase is the address the browser interface is reached at: --base when
// given, else the server a remote context talks to.
func webBase(flag string, target connect.Target) (string, error) {
	raw := strings.TrimSpace(flag)
	if raw == "" && target.Mode == connect.ModeRemote {
		raw = target.URL
	}
	if raw == "" {
		return "", core.Invalid("this context opens a local database, which has no web address; " +
			"pass --base with the address tix serve is reached at")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", core.Invalid("--base %q must be an http or https address", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}
