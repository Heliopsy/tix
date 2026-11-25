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
// operation of its own, hosted over SSH with task.show (Operation.Formats).
func taskLinkCmd(g *globals) *cobra.Command {
	var base, as string
	cmd := &cobra.Command{
		Use:   "link REF",
		Short: "Print a task's address in the browser interface",
		Long: "Print the absolute address of a task's screen in the browser interface, the link the " +
			"task screen's Share menu copies. The address is --base when given, else the task's " +
			"web_url, which a server configured with server.public_url sets, else the server this " +
			"context talks to. A local database without server.public_url has no web address, so it " +
			"needs --base naming the address tix serve is reached at. Over ssh the address is always " +
			"the server's web_url, and --base is refused.\n\n" +
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
			if isHosted(cmd) && cmd.Flags().Changed("base") {
				return usagef(cmd, "--base names an address on your behalf and is refused over ssh; "+
					"the link is the server's own web address (server.public_url)")
			}
			flag, err := checkBase(base)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			task, err := conn.Service.GetTask(ctx, ref)
			if err != nil {
				return err
			}
			address, err := linkAddress(flag, *task, conn.Info.Target, isHosted(cmd))
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

// newTaskLink builds every form of the link to t at address.
func newTaskLink(address string, t core.Task) taskLink {
	ref := output.ShowRef(t.Ref)
	return taskLink{Ref: ref, Title: t.Title, URL: address,
		Markdown: tasklink.Markdown(ref, t.Title, address), Text: tasklink.Text(ref, t.Title)}
}

// checkBase validates --base, returning it without its trailing slash.
func checkBase(flag string) (string, error) {
	raw := strings.TrimSpace(flag)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", core.Invalid("--base %q must be an http or https address", raw)
	}
	return strings.TrimRight(raw, "/"), nil
}

// linkAddress is the task's address in the browser interface: under --base
// when given, else the web_url the service set, else under the server a
// remote context talks to.
func linkAddress(base string, t core.Task, target connect.Target, hosted bool) (string, error) {
	switch {
	case base != "":
		return tasklink.URL(base, t.Ref), nil
	case t.WebURL != "":
		return t.WebURL, nil
	case hosted:
		return "", core.Invalid("this server knows no web address to link to: its operator sets " +
			"server.public_url (TIX_SERVER_PUBLIC_URL); tix task show prints the reference and title")
	case target.Mode == connect.ModeRemote && target.URL != "":
		return tasklink.URL(strings.TrimRight(strings.TrimSpace(target.URL), "/"), t.Ref), nil
	default:
		return "", core.Invalid("this context opens a local database, which has no web address; " +
			"pass --base with the address tix serve is reached at, or set server.public_url")
	}
}
