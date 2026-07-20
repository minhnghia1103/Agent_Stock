package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"agent_stock/internal/config"
	"agent_stock/internal/skills"
)

func skillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "List workspace skills (Phase 9)",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Print skill metadata",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintf(os.Stderr, "config: %v\n", err)
				os.Exit(1)
			}
			if err := skills.SeedIfMissing(cfg.WorkspacePath); err != nil {
				fmt.Fprintf(os.Stderr, "seed: %v\n", err)
				os.Exit(1)
			}
			reg := skills.NewRegistry(cfg.WorkspacePath, "")
			if cfg.SkillsPath != "" {
				reg = skills.NewRegistryWithDir(cfg.SkillsPath)
			}
			if err := reg.Reload(); err != nil {
				fmt.Fprintf(os.Stderr, "load skills: %v\n", err)
				os.Exit(1)
			}
			list := reg.List()
			if len(list) == 0 {
				fmt.Println("No skills found.")
				return
			}
			for _, sk := range list {
				fmt.Printf("/%-20s  %s\n", sk.Slug, sk.Description)
			}
		},
	})
	return cmd
}
