package cmd

import (
	"bytes"
	"embed"
	"io"
	"io/fs"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

//go:embed kube/templates/*.tmpl.yaml
var kubeTemplates embed.FS

// Manifest represents the data passed to the Kubernetes templates.
type Manifest struct {
	Name             string
	Image            string
	Command          []string
	Port             int
	SecretName       string
	SecretData       map[string]string
	UsesSharedConfig bool
}

var (
	outDir string
	env    string
)

var kubeCmd = &cobra.Command{
	Use:   "kube",
	Short: "Generate Kubernetes manifests for all agents and tools",
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			fh  fs.File
			buf bytes.Buffer
		)

		for _, embeddedFile := range []string{"app", "deployment", "service", "configmap", "secret"} {
			if fh, err = manifests.Open("kube/templates/" + embeddedFile + ".tmpl.yaml"); err != nil {
				log.Error("failed to open embedded config file", "error", err)
				return err
			}

			if _, err = io.Copy(&buf, fh); err != nil {
				log.Error("failed to read embedded config file", "error", err)
				return err
			}

			buf.Reset()
			fh.Close()
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(kubeCmd)
	kubeCmd.Flags().StringVarP(&outDir, "out", "o", "", "directory to write generated manifests (defaults to stdout)")
	kubeCmd.Flags().StringVar(&env, "env", "", "environment name to render (currently unused)")
}
