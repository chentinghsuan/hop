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
	_, err := findMongoBin()
	return err
}

func findMongoBin() (string, error) {
	if bin, err := exec.LookPath("mongosh"); err == nil {
		return bin, nil
	}
	if bin, err := exec.LookPath("mongo"); err == nil {
		return bin, nil
	}
	return "", errors.New("neither 'mongosh' nor legacy 'mongo' CLI was found. Install it via 'brew install mongosh' or on Windows 'winget install MongoDB.Server'")
}

func (c *MongoConnector) Connect(ctx context.Context, target config.ResolvedTarget, resolver secret.Resolver) error {
	mongoBin, err := findMongoBin()
	if err != nil {
		return err
	}

	// 1. Resolve user and password from Secret Provider or fallback properties
	user, password, err := ResolveCredentials(ctx, "Mongo", target, resolver, "")
	if err != nil {
		return err
	}

	// 2. Build Connection String
	connURI := GetString(target.Properties, "uri", "")
	if connURI != "" {
		// Custom template: replace {{PASSWORD}} and {{USER}}
		if password != "" {
			connURI = strings.ReplaceAll(connURI, "{{PASSWORD}}", url.QueryEscape(password))
		}
		if user != "" {
			connURI = strings.ReplaceAll(connURI, "{{USER}}", url.QueryEscape(user))
		}
	} else {
		host := GetString(target.Properties, "host", "localhost")
		port := GetInt(target.Properties, "port", 27017)
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

	args = append(args, GetStringSlice(target.Properties, "extra_args")...)

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
