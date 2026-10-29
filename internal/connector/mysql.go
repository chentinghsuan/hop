package connector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/fatih/color"
	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/secret"
)

type MySQLConnector struct{}

func NewMySQLConnector() *MySQLConnector {
	return &MySQLConnector{}
}

func (c *MySQLConnector) Type() string {
	return "mysql"
}

func (c *MySQLConnector) Aliases() []string {
	return []string{"mariadb", "sql", "proxysql", "proxy"}
}

func (c *MySQLConnector) ValidateDependencies() error {
	if _, err := exec.LookPath("mysql"); err != nil {
		return errors.New("'mysql' client CLI is not found. Install it via 'brew install mysql-client'")
	}
	return nil
}

func (c *MySQLConnector) Connect(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) error {
	if err := c.ValidateDependencies(); err != nil {
		return err
	}

	host := GetString(target.Properties, "host", "127.0.0.1")
	port := GetInt(target.Properties, "port", 3306)
	user := GetString(target.Properties, "user", "")
	dbName := GetString(target.Properties, "database", "")

	// 1. Resolve password from Secret Provider
	var password string
	if target.Secret.Item != "" {
		color.HiBlack("Fetching MySQL credentials from [%s]...", target.Secret.Provider)
		var err error
		password, err = resolver.Resolve(ctx, target.Secret)
		if err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("retrieved password from [%s] for item %q is empty! Please check that the Password field in Bitwarden is filled",
				target.Secret.Provider, target.Secret.Item)
		}

		// If user is not set, try to get username from secret
		if user == "" {
			userSec := target.Secret
			userSec.Field = "username"
			if u, err := resolver.Resolve(ctx, userSec); err == nil && u != "" {
				user = u
			}
		}
	} else if pwd := GetString(target.Properties, "password", ""); pwd != "" {
		password = pwd
	}

	if user == "" {
		user = "root"
	}

	args := []string{
		"-h", host,
		"-P", strconv.Itoa(port),
		"-u", user,
	}

	if defaultAuth := GetString(target.Properties, "default_auth", ""); defaultAuth != "" {
		args = append(args, fmt.Sprintf("--default-auth=%s", defaultAuth))
	}

	if sslMode := GetString(target.Properties, "ssl_mode", ""); sslMode != "" {
		args = append(args, fmt.Sprintf("--ssl-mode=%s", sslMode))
	}

	if dbName != "" {
		args = append(args, dbName)
	}

	// Extra args if any
	if extra, ok := target.Properties["extra_args"].([]any); ok {
		for _, a := range extra {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}

	color.Cyan("Connecting to MySQL: %s@%s:%d/%s...", user, host, port, dbName)

	binPath, err := exec.LookPath("mysql")
	if err != nil {
		return errors.New("'mysql' client CLI is not found. Install it via 'brew install mysql-client' or on Windows 'winget install Oracle.MySQL'")
	}

	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Inject password via environment variable MYSQL_PWD to keep it out of process arguments
	cmd.Env = os.Environ()
	if password != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("MYSQL_PWD=%s", password))
	}

	return cmd.Run()
}
