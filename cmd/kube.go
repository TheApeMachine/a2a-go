package cmd

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		manifests, err := generateManifests()
		if err != nil {
			return err
		}

		// Sort keys for deterministic output
		keys := make([]string, 0, len(manifests))
		for k := range manifests {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		if outDir == "" {
			// Print to stdout
			for i, k := range keys {
				fmt.Print(string(manifests[k]))
				if i < len(keys)-1 {
					fmt.Print("---\n")
				}
			}
			return nil
		}

		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		for _, k := range keys {
			filename := filepath.Join(outDir, fmt.Sprintf("%s.yaml", k))
			if err := os.WriteFile(filename, manifests[k], 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(kubeCmd)
	kubeCmd.Flags().StringVarP(&outDir, "out", "o", "", "directory to write generated manifests (defaults to stdout)")
	kubeCmd.Flags().StringVar(&env, "env", "", "environment name to render (currently unused)")
}

func nindent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return pad + strings.ReplaceAll(v, "\n", "\n"+pad)
}

func generateManifests() (map[string][]byte, error) {
	result := make(map[string][]byte)

	// Image tag can later be switched based on env flag
	const defaultImage = "docker.io/theapemachine/a2a-go:latest"

	endpoints := viper.GetStringMapString("endpoints")
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints found in config")
	}

	funcMap := template.FuncMap{
		"nindent": nindent,
	}

	// Load templates once
	deploymentTmpl, err := template.New("deployment.tmpl.yaml").Funcs(funcMap).ParseFS(kubeTemplates, "kube/templates/deployment.tmpl.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse deployment template: %w", err)
	}
	serviceTmpl, err := template.New("service.tmpl.yaml").Funcs(funcMap).ParseFS(kubeTemplates, "kube/templates/service.tmpl.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse service template: %w", err)
	}
	secretTmpl, err := template.New("secret.tmpl.yaml").Funcs(funcMap).ParseFS(kubeTemplates, "kube/templates/secret.tmpl.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse secret template: %w", err)
	}
	configmapTmpl, err := template.New("configmap.tmpl.yaml").Funcs(funcMap).ParseFS(kubeTemplates, "kube/templates/configmap.tmpl.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to parse configmap template: %w", err)
	}

	// Generate shared ConfigMap and add it to the results.
	configData, err := os.ReadFile("cmd/cfg/config.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to read config.yml: %w", err)
	}

	var configBuf bytes.Buffer
	err = configmapTmpl.Execute(&configBuf, map[string]interface{}{
		"ConfigData": string(configData),
	})
	if err != nil {
		return nil, fmt.Errorf("configmap template exec error: %w", err)
	}
	result["a2a_go_configmap"] = configBuf.Bytes()

	for name := range endpoints {
		// Skip special entries that are not services
		if strings.Contains(name, "Path") {
			continue
		}

		secretName, secretData := getSecretForComponent(name)
		m := Manifest{
			Name:             name,
			Image:            defaultImage,
			Port:             3210,
			Command:          deriveCommand(name),
			SecretName:       secretName,
			SecretData:       secretData,
			UsesSharedConfig: true,
		}

		// Render deployment
		var buf bytes.Buffer
		if m.SecretName != "" {
			if err := secretTmpl.Execute(&buf, m); err != nil {
				return nil, fmt.Errorf("secret template exec error for %s: %w", name, err)
			}
		}
		if err := deploymentTmpl.Execute(&buf, m); err != nil {
			return nil, fmt.Errorf("deploy template exec error for %s: %w", name, err)
		}
		// Render service right after
		if err := serviceTmpl.Execute(&buf, m); err != nil {
			return nil, fmt.Errorf("service template exec error for %s: %w", name, err)
		}

		result[name] = buf.Bytes()
	}

	return result, nil
}

func getSecretForComponent(name string) (string, map[string]string) {
	secretName := strings.ReplaceAll(name, "_", "-") + "-secret"

	// Azure tools share the same secret structure.
	if strings.HasPrefix(name, "azure_") {
		return secretName, map[string]string{
			"AZURE_DEVOPS_ORG":     "your-org",
			"AZDO_PAT":             "your-pat",
			"AZURE_DEVOPS_PROJECT": "your-project",
			"AZURE_DEVOPS_TEAM":    "your-team",
		}
	}

	// Tools requiring Slack tokens.
	switch name {
	case "delegatetool", "slack", "browsertool", "catalogtool", "catalog", "webhook":
		return secretName, map[string]string{
			"SLACK_APP_TOKEN":  "xapp-your-token",
			"SLACK_BOT_TOKEN":  "xoxb-your-token",
			"SLACK_USER_TOKEN": "xoxu-your-token",
		}
	}

	// No secret needed for this component.
	return "", nil
}

func deriveCommand(name string) []string {
	// Direct binaries that are not executed via mcp.
	switch name {
	case "catalog", "webhook", "slack":
		return []string{name}
	}

	// Common pattern: browsertool -> mcp -c browser
	if strings.HasSuffix(name, "tool") {
		base := strings.TrimSuffix(name, "tool")
		return []string{"mcp", "-c", base}
	}

	// Fallback: use the name directly as the config flag
	return []string{"mcp", "-c", name}
}
