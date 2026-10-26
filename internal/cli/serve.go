package cli

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/mdbtq/dotclaude/internal/server"
	"github.com/mdbtq/dotclaude/web"
)

func serveCmd(flags *pathFlags) *cobra.Command {
	var (
		port     int
		open     bool
		readOnly bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the web UI on localhost",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := flags.paths()
			if err != nil {
				return err
			}
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return err
			}
			srv := &server.Server{Paths: p, Version: cmd.Root().Version, UI: web.Dist(), ReadOnly: readOnly}
			url := fmt.Sprintf("http://localhost:%d", ln.Addr().(*net.TCPAddr).Port)
			fmt.Fprintf(cmd.OutOrStdout(), "dotclaude is running at %s (ctrl+c to stop)\n", url)
			if srv.UI == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: this build has no web UI; only the API is available")
			}
			if open {
				openBrowser(url)
			}
			httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
			return httpSrv.Serve(ln)
		},
	}
	cmd.Flags().IntVar(&port, "port", 7878, "port to listen on (0 picks a free one)")
	cmd.Flags().BoolVar(&open, "open", false, "open the UI in the default browser")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "refuse every change; the UI only shows")
	return cmd
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
