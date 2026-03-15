package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/config"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	loginEmail    string
	loginPassword bool
	loginAPIKey   bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Clank platform",
	Long: `Log in to your Clank instance.

By default, opens your browser for authentication (supports all sign-in
methods: magic link, passkey, OAuth, password).

Alternatives:
  clank login --password    Email & password (for headless/CI environments)
  clank login --api-key     Paste an existing API key`,
	RunE: runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	switch {
	case loginPassword:
		return runPasswordLogin()
	case loginAPIKey:
		return runAPIKeyLogin()
	default:
		return runBrowserLogin()
	}
}

// runBrowserLogin uses the device authorization flow.
// Works with any auth method (magic link, OAuth, passkey, password).
func runBrowserLogin() error {
	client := api.New(cfg.BaseURL, "")

	// 1. Request device code
	resp, err := api.RequestDeviceCode(client)
	if err != nil {
		return fmt.Errorf("failed to start login: %w\n\nTip: use --password for email/password login", err)
	}

	// 2. Show code and open browser
	fmt.Println()
	fmt.Printf("  Your code: %s\n", resp.UserCode)
	fmt.Println()
	fmt.Printf("  Opening %s in your browser...\n", resp.VerificationURL)
	fmt.Println("  Enter the code above to authorize this device.")
	fmt.Println()

	url := resp.VerificationURL + "?code=" + resp.UserCode
	if err := browser.OpenURL(url); err != nil {
		fmt.Printf("  Could not open browser. Visit this URL manually:\n")
		fmt.Printf("  %s\n\n", url)
	}

	// 3. Poll for authorization
	interval := time.Duration(resp.Interval) * time.Second
	if interval < 3*time.Second {
		interval = 3 * time.Second
	}
	deadline := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)

	fmt.Print("  Waiting for authorization...")

	for time.Now().Before(deadline) {
		time.Sleep(interval)

		tokenResp, err := api.PollDeviceToken(client, resp.DeviceCode)
		if err != nil {
			// Check if it's an expired token error
			if apiErr, ok := err.(*api.APIError); ok && apiErr.Detail == "expired_token" {
				fmt.Println(" expired")
				return fmt.Errorf("authorization expired — please try again")
			}
			return fmt.Errorf("login failed: %w", err)
		}

		if tokenResp == nil {
			// Still pending
			fmt.Print(".")
			continue
		}

		// Success!
		fmt.Println(" done!")
		fmt.Println()

		if err := config.SaveToken(tokenResp.AccessToken); err != nil {
			return fmt.Errorf("saving token: %w", err)
		}

		fmt.Printf("  Logged in as %s\n", tokenResp.User.Email)
		return nil
	}

	fmt.Println(" timed out")
	return fmt.Errorf("authorization timed out — please try again")
}

// runPasswordLogin is the original email/password flow.
func runPasswordLogin() error {
	email := loginEmail
	if email == "" {
		fmt.Print("Email: ")
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading email: %w", err)
		}
		email = strings.TrimSpace(line)
	}

	if email == "" {
		return fmt.Errorf("email is required")
	}

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	password := string(passwordBytes)

	if password == "" {
		return fmt.Errorf("password is required")
	}

	client := api.New(cfg.BaseURL, "")
	resp, token, err := api.Login(client, email, password)
	if err != nil {
		if api.IsUnauthorized(err) {
			return fmt.Errorf("invalid email or password")
		}
		return fmt.Errorf("login failed: %w", err)
	}

	if err := config.SaveToken(token); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Printf("Logged in as %s\n", resp.User.Email)
	return nil
}

// runAPIKeyLogin lets users paste an API key directly.
func runAPIKeyLogin() error {
	fmt.Print("API Key: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading API key: %w", err)
	}
	key := strings.TrimSpace(line)

	if key == "" {
		return fmt.Errorf("API key is required")
	}

	if !strings.HasPrefix(key, "clank_") {
		return fmt.Errorf("invalid API key format (should start with 'clank_')")
	}

	// Validate the key by calling /api/auth/me
	client := api.New(cfg.BaseURL, key)
	user, err := api.Me(client)
	if err != nil {
		if api.IsUnauthorized(err) {
			return fmt.Errorf("invalid API key")
		}
		return fmt.Errorf("validating API key: %w", err)
	}

	if err := config.SaveToken(key); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}

	fmt.Printf("Logged in as %s\n", user.Email)
	return nil
}

func init() {
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "email address (for --password mode)")
	loginCmd.Flags().BoolVar(&loginPassword, "password", false, "use email/password authentication")
	loginCmd.Flags().BoolVar(&loginAPIKey, "api-key", false, "authenticate with an API key")
	rootCmd.AddCommand(loginCmd)
}
