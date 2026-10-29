package connector

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/ting-hsuanchen/hop/internal/config"
	"github.com/ting-hsuanchen/hop/internal/secret"
)

type MongoConnector struct{}

func NewMongoConnector() *MongoConnector {
	return &MongoConnector{}
}

func (c *MongoConnector) Type() string {
	return "mongo"
}

func (c *MongoConnector) Aliases() []string {
	return []string{"mongodb", "atlas"}
}

func (c *MongoConnector) ValidateDependencies() error {
	if _, err := exec.LookPath("mongosh"); err == nil {
		return nil
	}
	if _, err := exec.LookPath("mongo"); err == nil {
		return nil
	}
	return errors.New("neither 'mongosh' nor legacy 'mongo' CLI was found. Install it via 'brew install mongosh'")
}

func (c *MongoConnector) Connect(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) error {
	if err := c.ValidateDependencies(); err != nil {
		return err
	}

	mongoBin, err := exec.LookPath("mongosh")
	if err != nil {
		mongoBin, err = exec.LookPath("mongo")
		if err != nil {
			return errors.New("neither 'mongosh' nor legacy 'mongo' CLI was found. Install it via 'brew install mongosh' or on Windows 'winget install MongoDB.Server'")
		}
	}

	// 1. Resolve password from Secret Provider
	var password string
	if target.Secret.Item != "" {
		color.HiBlack("Fetching Mongo credentials from [%s]...", target.Secret.Provider)
		var err error
		password, err = resolver.Resolve(ctx, target.Secret)
		if err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("retrieved password from [%s] for item %q is empty! Please check that the Password field in Bitwarden is filled",
				target.Secret.Provider, target.Secret.Item)
		}
	} else if pwd := GetString(target.Properties, "password", ""); pwd != "" {
		password = pwd
	}

	user := GetString(target.Properties, "user", "")
	// If user is empty, attempt to fetch username from Secret Provider
	if user == "" && target.Secret.Item != "" {
		userSec := target.Secret
		userSec.Field = "username"
		if u, err := resolver.Resolve(ctx, userSec); err == nil && u != "" {
			user = u
		}
	}

	// 2. Build Connection String
	connURI := GetString(target.Properties, "uri", "")
	if connURI != "" {
		// Custom template: replace {{PASSWORD}}
		if password != "" {
			escapedPwd := url.QueryEscape(password)
			connURI = strings.ReplaceAll(connURI, "{{PASSWORD}}", escapedPwd)
		}
	} else {
		host := GetString(target.Properties, "host", "localhost")
		port := GetInt(target.Properties, "port", 27017)
		user := GetString(target.Properties, "user", "")
		dbName := GetString(target.Properties, "database", "")
		authSource := GetString(target.Properties, "auth_source", "")
		isSRV := GetBool(target.Properties, "srv", false)

		var userInfo string
		if user != "" {
			if password != "" {
				userInfo = fmt.Sprintf("%s:%s@", url.QueryEscape(user), url.QueryEscape(password))
			} else {
				userInfo = fmt.Sprintf("%s@", url.QueryEscape(user))
			}
		}

		scheme := "mongodb"
		hostPort := fmt.Sprintf("%s:%s", host, strconv.Itoa(port))
		if isSRV {
			scheme = "mongodb+srv"
			hostPort = host // SRV records do not take port
		}

		connURI = fmt.Sprintf("%s://%s%s/%s", scheme, userInfo, hostPort, dbName)

		var queryParams []string
		if authSource != "" {
			queryParams = append(queryParams, fmt.Sprintf("authSource=%s", url.QueryEscape(authSource)))
		}

		if len(queryParams) > 0 {
			connURI += "?" + strings.Join(queryParams, "&")
		}
	}

	var args []string
	if password != "" {
		// Secure connection: Avoid passing connection string with plaintext password via CLI arguments,
		// which would expose credentials in `ps aux` and `/proc/<pid>/cmdline`.
		// Instead, inject via environment variable HOP_MONGO_URI and connect via evaluation.
		evalScript := "try { db = connect((typeof process !== 'undefined' && process.env) ? process.env.HOP_MONGO_URI : _getEnv('HOP_MONGO_URI')); } catch (e) { console.error('Connection failed:', e.message); quit(1); }"
		args = append(args, "--nodb", "--eval", evalScript, "--shell")
	} else {
		args = append(args, connURI)
	}

	// Extra args if any
	if extra, ok := target.Properties["extra_args"].([]any); ok {
		for _, a := range extra {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}

	// Mask password in display message
	displayURI := maskMongoURI(connURI)
	color.Cyan("Connecting to MongoDB: %s...", displayURI)

	cmd := exec.CommandContext(ctx, mongoBin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = os.Environ()
	if password != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("HOP_MONGO_URI=%s", connURI))
	}

	return cmd.Run()
}

func maskMongoURI(rawURI string) string {
	u, err := url.Parse(rawURI)
	if err != nil {
		return rawURI
	}
	if u.User != nil {
		if _, hasPwd := u.User.Password(); hasPwd {
			u.User = url.UserPassword(u.User.Username(), "REDACTED")
			return strings.ReplaceAll(u.String(), "REDACTED", "******")
		}
	}
	return u.String()
}
