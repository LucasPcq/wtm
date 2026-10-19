package main

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/web"
	"github.com/spf13/cobra"
)

// serve is the spike's command: the catalogue over local HTTP, to one page.
func serve(env cli.Env) *cobra.Command {
	cmd := &cobra.Command{Use: "serve", Short: "Spike: serve the commands to a local web page", Args: cobra.NoArgs}
	addr := cmd.Flags().String("addr", "127.0.0.1:7777", "the address to listen on")

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		listener, err := net.Listen("tcp", *addr)
		if err != nil {
			return err
		}

		secret := make([]byte, 16)
		_, _ = rand.Read(secret)
		token := hex.EncodeToString(secret)
		dir := cmp.Or(env.ProjectDir, env.Cwd)

		server := &http.Server{Handler: web.Handler(web.Params{Catalogue: catalogue(), Dir: dir, Token: token})}
		go func() {
			<-cmd.Context().Done()
			_ = server.Close()
		}()

		fmt.Fprintf(cmd.OutOrStdout(), "wtm-next serves %s on http://%s/#token=%s\n", dir, listener.Addr(), token)
		if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	return cmd
}
