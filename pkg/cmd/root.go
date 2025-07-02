package cmd

import (
	"fmt"
	"github.com/defektive/requrse/pkg/request"
	"github.com/spf13/cobra"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "requrse",
	Short: "Send HTTP requests until specific conditions are met",
	Long:  `Send HTTP requests until specific conditions are met.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		host, _ := cmd.Flags().GetString("host")
		auth, _ := cmd.Flags().GetString("auth")
		template, _ := cmd.Flags().GetString("template")
		outputDir, _ := cmd.Flags().GetString("out")
		ext, _ := cmd.Flags().GetString("ext")
		extra, _ := cmd.Flags().GetStringSlice("extra")
		lists, _ := cmd.Flags().GetStringSlice("list")
		mode, _ := cmd.Flags().GetString("mode")

		req, err := request.FromFile(template)
		if err != nil {
			panic(err)
		}

		extraData := map[string]interface{}{}

		for _, value := range extra {
			i := strings.Index(value, "=")
			extraData[value[:i]] = value[i+1:]
		}

		c := &request.RequestContext{
			Host:      host,
			AuthToken: auth,
			Extra:     extraData,
		}

		if outputDir != "" {
			err = os.MkdirAll(outputDir, 0755)
			if err != nil {
				panic(err)
			}
		}

		if len(lists) > 0 {
			if mode == "pitchfork" {
				for _, list := range lists {
					fileBytes, err := os.ReadFile(filepath.Join(list))
					if err != nil {
						panic(err)
					}
					req.Lists = append(req.Lists, strings.Split(string(fileBytes), "\n"))
				}
			}

		}

		iteration := 0
		req.Recurse(c, func(body []byte) {
			if outputDir != "" {
				err := os.WriteFile(filepath.Join(outputDir, fmt.Sprintf("response-%d.%s", iteration, ext)), body, 0644)
				if err != nil {
					log.Println(err)
				}
			} else {
				fmt.Println(string(body))
			}
			iteration++
		})

		//log.Println(iteration)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("template", "t", "", "Template to process")
	rootCmd.PersistentFlags().StringP("host", "H", "localhost", "http host")
	rootCmd.PersistentFlags().StringP("auth", "a", "", "auth token")
	rootCmd.PersistentFlags().StringP("out", "o", "", "output directory")
	rootCmd.PersistentFlags().String("ext", "json", "extension for files in output directory")
	rootCmd.PersistentFlags().StringSliceP("extra", "e", []string{}, "extra data (-e something=someval)")
	rootCmd.PersistentFlags().StringSliceP("list", "l", []string{}, "list files (-l wordlist-01 -l wordlist-02)")

	rootCmd.PersistentFlags().StringP("mode", "m", "", "Mode for list usage. Currently only Pitchfork")

}
