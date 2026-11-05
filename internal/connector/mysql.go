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
	_, err := findMySQLBin()
	return err
}

func findMySQLBin() (string, error) {
	binPath, err := exec.LookPath("mysql")
	if err != nil {
		return "", errors.New("'mysql' client CLI is not found. Install it via 'brew install mysql-client' or on Windows 'winget install Oracle.MySQL'")
	}
	return binPath, nil
}

func (c *MySQLConnector) Connect(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) error {
	binPath, err := findMySQLBin()
	if err != nil {
		return err
	}

	host := GetString(target.Properties, "host", "127.0.0.1")
	port := GetInt(target.Properties, "port", 3306)
	dbName := GetString(target.Properties, "database", "")

	user, password, err := ResolveCredentials(ctx, "MySQL", target, resolver, "root")
	if err != nil {
		return err
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

	if sslCA := GetString(target.Properties, "ssl_ca", ""); sslCA != "" {
		args = append(args, fmt.Sprintf("--ssl-ca=%s", sslCA))
	}

	if GetBool(target.Properties, "comments", false) {
		args = append(args, "--comments")
	}

	args = append(args, GetStringSlice(target.Properties, "extra_args")...)

	if dbName != "" {
		args = append(args, dbName)
	}

	color.Cyan("Connecting to MySQL: %s@%s:%d/%s...", user, host, port, dbName)

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
