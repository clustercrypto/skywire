// Package clivisor cmd/skywire-cli/commands/visor/info.go
package clivisor

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	clirpc "github.com/skycoin/skywire/cmd/skywire-cli/commands/rpc"
	"github.com/skycoin/skywire/cmd/skywire-cli/internal"
	"github.com/skycoin/skywire/pkg/visor/visorconfig"
)

var path string
var pkg bool
var web bool
var webPort string
var pk string

func init() {
	RootCmd.AddCommand(pkCmd)
	pkCmd.Flags().StringVarP(&path, "input", "i", "", "path of input config file.")
	pkCmd.Flags().BoolVarP(&pkg, "pkg", "p", false, "read from "+fmt.Sprintf("%s", visorconfig.PackageConfig())) //nolint
	pkCmd.Flags().BoolVarP(&web, "http", "w", false, "serve public key via http")
	pkCmd.Flags().StringVarP(&webPort, "prt", "x", "7998", "serve public key via http")
	RootCmd.AddCommand(summaryCmd)
	RootCmd.AddCommand(buildInfoCmd)
	RootCmd.AddCommand(portsCmd)
}

var pkCmd = &cobra.Command{
	Use:   "pk",
	Short: "Public key of the visor",
	Long:  "\n  Public key of the visor",
	Run: func(cmd *cobra.Command, _ []string) {
		if pkg {
			path = visorconfig.SkywireConfig()
		}
		var outputPK string
		if path != "" {
			conf, err := visorconfig.ReadFile(path)
			if err != nil {
				internal.PrintFatalError(cmd.Flags(), fmt.Errorf("Failed to read config: %v", err))
			}
			outputPK = conf.PK.Hex()
		} else {
			rpcClient, err := clirpc.Client(cmd.Flags())
			if err != nil {
				os.Exit(1)
			}
			overview, err := rpcClient.Overview()
			if err != nil {
				internal.PrintFatalRPCError(cmd.Flags(), err)
			}
			pk = overview.PubKey.String() + "\n"
			if web {
				http.HandleFunc("/", srvpk)
				logger.Info("\nServing public key " + pk + " on port " + webPort)
				http.ListenAndServe(":"+webPort, nil) //nolint
			}
			outputPK = overview.PubKey.Hex() + "\n"
		}
		internal.PrintOutput(cmd.Flags(), strings.TrimSuffix(outputPK, "\n"), outputPK)
	},
}

var summaryCmd = &cobra.Command{
	Use:   "info",
	Short: "Summary of visor info",
	Long:  "\n  Summary of visor info",
	Run: func(cmd *cobra.Command, _ []string) {
		rpcClient, err := clirpc.Client(cmd.Flags())
		if err != nil {
			os.Exit(1)
		}
		summary, err := rpcClient.Summary()
		if err != nil {
			internal.PrintFatalRPCError(cmd.Flags(), err)
		}
		msg := fmt.Sprintf(".:: Visor Summary ::.\nPublic key: %q\nSymmetric NAT: %t\nIP: %s\nDMSG Server: %q\nPing: %q\nVisor Version: %s\nSkybian Version: %s\nUptime Tracker: %s\nTime Online: %f seconds\nBuild Tag: %s\n",
			summary.Overview.PubKey, summary.Overview.IsSymmetricNAT, summary.Overview.LocalIP, summary.DmsgStats.ServerPK, summary.DmsgStats.RoundTrip, summary.Overview.BuildInfo.Version, summary.SkybianBuildVersion,
			summary.Health.ServicesHealth, summary.Uptime, summary.BuildTag)

		outputJSON := struct {
			PublicKey      string  `json:"public_key"`
			IsSymmetricNAT bool    `json:"symmetric_nat"`
			IP             string  `json:"ip"`
			DmsgServer     string  `json:"dmsg_server"`
			Ping           string  `json:"ping"`
			VisorVersion   string  `json:"visor_version"`
			SkybianVersion string  `json:"skybian_version"`
			UptimeTracker  string  `json:"uptime_tracker"`
			TimeOnline     float64 `json:"time_online"`
			BuildTag       string  `json:"build_tag"`
		}{
			PublicKey:      summary.Overview.PubKey.String(),
			IsSymmetricNAT: summary.Overview.IsSymmetricNAT,
			IP:             summary.Overview.LocalIP,
			DmsgServer:     summary.DmsgStats.ServerPK.String(),
			Ping:           summary.DmsgStats.RoundTrip.String(),
			VisorVersion:   summary.Overview.BuildInfo.Version,
			SkybianVersion: summary.SkybianBuildVersion,
			UptimeTracker:  summary.Health.ServicesHealth,
			TimeOnline:     summary.Uptime,
			BuildTag:       summary.BuildTag,
		}
		internal.PrintOutput(cmd.Flags(), outputJSON, msg)
	},
}

var buildInfoCmd = &cobra.Command{
	Use:   "ver",
	Short: "Version and build info",
	Long:  "\n  Version and build info",
	Run: func(cmd *cobra.Command, _ []string) {
		rpcClient, err := clirpc.Client(cmd.Flags())
		if err != nil {
			os.Exit(1)
		}
		overview, err := rpcClient.Overview()
		if err != nil {
			internal.PrintFatalRPCError(cmd.Flags(), err)
		}
		buildInfo := overview.BuildInfo
		msg := fmt.Sprintf("Version %q built on %q against commit %q\n", buildInfo.Version, buildInfo.Date, buildInfo.Commit)
		internal.PrintOutput(cmd.Flags(), buildInfo, msg)
	},
}

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List of Ports",
	Long:  "\n  List of all ports used by visor services and apps",
	Run: func(cmd *cobra.Command, _ []string) {
		rpcClient, err := clirpc.Client(cmd.Flags())
		if err != nil {
			os.Exit(1)
		}
		ports, err := rpcClient.Ports()
		if err != nil {
			internal.PrintFatalRPCError(cmd.Flags(), err)
		}
		msg := "+-------------------------------------------+\n"
		msg += fmt.Sprintf("| %-21s | %-7s | %-7s |\n", "App/Service", "Type", "Port")
		msg += "|-------------------------------------------|\n"

		portsName := make([]string, 0, len(ports))
		for portName := range ports {
			portsName = append(portsName, portName)
		}
		sort.Strings(portsName)

		for _, portName := range portsName {
			msg += fmt.Sprintf("| %-21s | %-7s | %7s |\n", portName, ports[portName].Type, ports[portName].Port)
		}

		msg += "|===========================================|\n"
		msg += "| SKYNET: connection between apps and visor |\n"
		msg += "| DMSG: connection by dmsg service          |\n"
		msg += "+-------------------------------------------+\n"
		internal.PrintOutput(cmd.Flags(), ports, msg)
	},
}

func srvpk(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, pk) //nolint
}
