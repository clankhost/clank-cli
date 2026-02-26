package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/anaremore/clank/apps/cli/internal/api"
	"github.com/anaremore/clank/apps/cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginEmail string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the Clank platform",
	Long:  "Log in with your email and password. The auth token is stored locally.",
	RunE:  runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
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
	fmt.Println() // newline after hidden input
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

func init() {
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "email address (prompted if not provided)")
	rootCmd.AddCommand(loginCmd)
}
