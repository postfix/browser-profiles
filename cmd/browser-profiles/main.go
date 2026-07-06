// Command browser-profiles is the self-hosted anti-detect browser-profiles CLI,
// a cobra port of src/cli.ts. It mirrors the seven TS commands (list/create/
// delete/info/open/launch/path) and their flags/behavior. connectPuppeteer is
// dropped (subsumed by go-rod's ControlURL().Connect()).
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	browserprofiles "github.com/postfix/browser-profiles"
)

// profiles is the single manager instance (default storage), matching the TS
// module-level `new BrowserProfiles()`.
var profiles = browserprofiles.NewBrowserProfiles(browserprofiles.BrowserProfilesOptions{})

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// newRootCmd builds the command tree. Split out from main so tests can construct the CLI,
// set args + output, and Execute() without spawning a process.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "browser-profiles",
		Short:         "Self-hosted anti-detect browser profiles CLI",
		Version:       browserprofiles.VERSION,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(listCmd(), createCmd(), deleteCmd(), infoCmd(), openCmd(), launchCmd(), pathCmd())
	return root
}

// fail prints "<prefix> <err>" to stderr and exits 1, mirroring the TS
// `console.error(prefix, err.message); process.exit(1)`.
func fail(prefix string, err error) {
	fmt.Fprintln(os.Stderr, prefix, err.Error())
	os.Exit(1)
}

// failMsg prints a bare message to stderr and exits 1.
func failMsg(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

// padTrunc truncates s to n runes then right-pads it to width n (JS
// substring(0,n).padEnd(n)).
func padTrunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return fmt.Sprintf("%-*s", n, string(r))
}

// localeDate approximates JS Date.toLocaleDateString() (en-US: M/D/YYYY).
func localeDate(millis int64) string {
	return time.UnixMilli(millis).Format("1/2/2006")
}

// localeDateTime approximates JS Date.toLocaleString().
func localeDateTime(millis int64) string {
	return time.UnixMilli(millis).Format("1/2/2006, 3:04:05 PM")
}

// parseProxyURL parses a proxy URL exactly like the TS CLI: scheme sans colon as
// type, hostname, port (default 8080), and optional user:pass. Note the m6
// cred-encoding quirk: url.User decodes percent-encoding (acceptable divergence).
func parseProxyURL(raw string) (*browserprofiles.ProxyConfig, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	port := 8080
	if v, err := strconv.Atoi(u.Port()); err == nil && v != 0 {
		port = v
	}
	pc := &browserprofiles.ProxyConfig{
		Type: u.Scheme,
		Host: u.Hostname(),
		Port: browserprofiles.Port(port),
	}
	if u.User != nil {
		pc.Username = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			pc.Password = pw
		}
	}
	return pc, nil
}

// useHeadlessShorthand reclaims -h for --headless: pre-registering a long-only
// --help flag stops cobra's InitDefaultHelpFlag from claiming -h for help.
func useHeadlessShorthand(cmd *cobra.Command) {
	cmd.Flags().Bool("help", false, "help for "+cmd.Name())
	cmd.Flags().BoolP("headless", "h", false, "Run in headless mode")
}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all browser profiles",
		Run: func(cmd *cobra.Command, _ []string) {
			asJSON, _ := cmd.Flags().GetBool("json")
			all, err := profiles.List(browserprofiles.ListOptions{})
			if err != nil {
				fail("Error listing profiles:", err)
			}

			if asJSON {
				out, _ := json.MarshalIndent(all, "", "  ")
				fmt.Println(string(out))
				return
			}

			if len(all) == 0 {
				fmt.Println("No profiles found. Create one with: browser-profiles create <name>")
				return
			}

			fmt.Print("\n📋 Browser Profiles:\n\n")
			fmt.Println("ID                                    | Name              | Proxy              | Created")
			fmt.Println("--------------------------------------|-------------------|--------------------|-----------------")
			for _, p := range all {
				name := p.Name
				if name == "" {
					name = "Unnamed"
				}
				proxy := "No proxy"
				if p.Proxy != nil {
					proxy = fmt.Sprintf("%s:%s", p.Proxy.Host, p.Proxy.Port.String())
				}
				fmt.Printf("%s | %s | %s | %s\n",
					padTrunc(p.ID, 36), padTrunc(name, 17), padTrunc(proxy, 18), localeDate(p.CreatedAt))
			}
			fmt.Printf("\nTotal: %d profile(s)\n\n", len(all))
		},
	}
	cmd.Flags().BoolP("json", "j", false, "Output as JSON")
	return cmd
}

func createCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new browser profile",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			id, _ := cmd.Flags().GetString("id")
			proxyURL, _ := cmd.Flags().GetString("proxy")
			timezone, _ := cmd.Flags().GetString("timezone")
			language, _ := cmd.Flags().GetString("language")
			platform, _ := cmd.Flags().GetString("platform")

			var proxy *browserprofiles.ProxyConfig
			if proxyURL != "" {
				p, err := parseProxyURL(proxyURL)
				if err != nil {
					fail("Error creating profile:", err)
				}
				proxy = p
			}

			profile, err := profiles.Create(browserprofiles.ProfileConfig{
				ID:       id,
				Name:     name,
				Proxy:    proxy,
				Timezone: timezone,
				Fingerprint: &browserprofiles.FingerprintConfig{
					Language: language,
					Platform: platform,
				},
			})
			if err != nil {
				fail("Error creating profile:", err)
			}

			fmt.Print("\n✅ Profile created successfully!\n\n")
			fmt.Printf("ID:       %s\n", profile.ID)
			fmt.Printf("Name:     %s\n", profile.Name)
			if proxy != nil {
				fmt.Printf("Proxy:    %s:%s\n", proxy.Host, proxy.Port.String())
			}
			if timezone != "" {
				fmt.Printf("Timezone: %s\n", timezone)
			}
			fmt.Printf("\nLaunch with: browser-profiles open %s\n", profile.ID)
			fmt.Printf("        or: browser-profiles open \"%s\"\n\n", profile.Name)
		},
	}
	cmd.Flags().StringP("id", "i", "", "Custom profile ID (alphanumeric + hyphen/underscore, 1-64 chars)")
	cmd.Flags().StringP("proxy", "p", "", "Proxy URL (e.g., http://user:pass@host:port)")
	cmd.Flags().StringP("timezone", "t", "", "Timezone (e.g., America/New_York)")
	cmd.Flags().StringP("language", "l", "", "Language (e.g., en-US)")
	cmd.Flags().String("platform", "", "Platform (Win32, MacIntel, Linux x86_64)")
	return cmd
}

func deleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <idOrName>",
		Aliases: []string{"rm"},
		Short:   "Delete a browser profile by ID or name",
		Args:    cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			// --force is declared but there is NO confirmation prompt (always
			// deletes), mirroring the TS behavior.
			profile, err := profiles.GetByIdOrName(args[0])
			if err != nil {
				fail("Error deleting profile:", err)
			}
			if profile == nil {
				failMsg(fmt.Sprintf("Profile not found: %s", args[0]))
			}

			ok, err := profiles.Delete(profile.ID)
			if err != nil {
				fail("Error deleting profile:", err)
			}
			if !ok {
				failMsg("Failed to delete profile")
			}

			label := profile.Name
			if label == "" {
				label = profile.ID
			}
			fmt.Printf("\n✅ Profile deleted: %s\n\n", label)
		},
	}
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	return cmd
}

func infoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <idOrName>",
		Short: "Show profile details by ID or name",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			asJSON, _ := cmd.Flags().GetBool("json")
			profile, err := profiles.GetByIdOrName(args[0])
			if err != nil {
				fail("Error getting profile:", err)
			}
			if profile == nil {
				failMsg(fmt.Sprintf("Profile not found: %s", args[0]))
			}

			if asJSON {
				out, _ := json.MarshalIndent(profile, "", "  ")
				fmt.Println(string(out))
				return
			}

			name := profile.Name
			if name == "" {
				name = "Unnamed"
			}
			fmt.Print("\n📋 Profile Details:\n\n")
			fmt.Printf("ID:         %s\n", profile.ID)
			fmt.Printf("Name:       %s\n", name)
			fmt.Printf("Created:    %s\n", localeDateTime(profile.CreatedAt))
			fmt.Printf("Updated:    %s\n", localeDateTime(profile.UpdatedAt))

			if profile.Proxy != nil {
				fmt.Println("\nProxy:")
				fmt.Printf("  Type:     %s\n", profile.Proxy.Type)
				fmt.Printf("  Host:     %s\n", profile.Proxy.Host)
				fmt.Printf("  Port:     %s\n", profile.Proxy.Port.String())
				if profile.Proxy.Username != "" {
					fmt.Printf("  Username: %s\n", profile.Proxy.Username)
				}
			}

			if profile.Timezone != "" {
				fmt.Printf("\nTimezone:   %s\n", profile.Timezone)
			}

			if fp := profile.Fingerprint; fp != nil {
				fmt.Println("\nFingerprint:")
				if fp.Language != "" {
					fmt.Printf("  Language: %s\n", fp.Language)
				}
				if fp.Platform != "" {
					fmt.Printf("  Platform: %s\n", fp.Platform)
				}
				if fp.UserAgent != "" {
					ua := []rune(fp.UserAgent)
					if len(ua) > 50 {
						ua = ua[:50]
					}
					fmt.Printf("  UA:       %s...\n", string(ua))
				}
			}
			fmt.Println("")
		},
	}
	cmd.Flags().BoolP("json", "j", false, "Output as JSON")
	return cmd
}

func openCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "open <idOrName>",
		Short: "Open browser with a profile (by ID or name)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			headless, _ := cmd.Flags().GetBool("headless")
			profile, err := profiles.GetByIdOrName(args[0])
			if err != nil {
				fail("Error launching browser:", err)
			}
			if profile == nil {
				failMsg(fmt.Sprintf("Profile not found: %s", args[0]))
			}

			label := profile.Name
			if label == "" {
				label = profile.ID
			}
			fmt.Printf("\n🚀 Launching browser for: %s\n", label)

			result, err := profiles.Launch(profile.ID, browserprofiles.LaunchOptions{Headless: headless})
			if err != nil {
				fail("Error launching browser:", err)
			}

			fmt.Println("\n✅ Browser launched!")
			fmt.Printf("   WebSocket: %s\n", result.WsEndpoint)
			fmt.Printf("   PID: %d\n", result.PID)
			fmt.Print("\nPress Ctrl+C to close the browser.\n\n")

			stayAlive(func() { _ = result.Close() })
		},
	}
	useHeadlessShorthand(cmd)
	return cmd
}

func launchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Quick launch browser without saving profile",
		Run: func(cmd *cobra.Command, _ []string) {
			proxyURL, _ := cmd.Flags().GetString("proxy")
			headless, _ := cmd.Flags().GetBool("headless")
			random, _ := cmd.Flags().GetBool("random")

			var proxy *browserprofiles.ProxyConfig
			if proxyURL != "" {
				p, err := parseProxyURL(proxyURL)
				if err != nil {
					fail("Error launching browser:", err)
				}
				proxy = p
			}

			fmt.Println("\n🚀 Quick launching browser...")

			session, err := browserprofiles.CreateSession(browserprofiles.CreateSessionOptions{
				Proxy:             proxy,
				Headless:          headless,
				RandomFingerprint: &random,
			})
			if err != nil {
				fail("Error launching browser:", err)
			}

			fmt.Println("\n✅ Browser launched!")
			fmt.Printf("   Session: %s\n", session.ID)
			fmt.Print("\nPress Ctrl+C to close the browser.\n\n")

			stayAlive(func() { _ = session.Close() })
		},
	}
	cmd.Flags().StringP("proxy", "p", "", "Proxy URL")
	cmd.Flags().Bool("random", true, "Use random fingerprint")
	useHeadlessShorthand(cmd)
	return cmd
}

func pathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show profiles storage path",
		Run: func(_ *cobra.Command, _ []string) {
			// Mirror the TS `process.env.HOME + '/.aitofy/browser-profiles'`
			// literally (do not "fix" a missing HOME).
			fmt.Printf("\n📁 Profiles stored at: %s\n\n", os.Getenv("HOME")+"/.aitofy/browser-profiles")
		},
	}
}

// stayAlive blocks until SIGINT, then runs closeFn and exits 0 (the TS
// process.on('SIGINT') stay-alive loop).
func stayAlive(closeFn func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	fmt.Println("\nClosing browser...")
	closeFn()
	os.Exit(0)
}
