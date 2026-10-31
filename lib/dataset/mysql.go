// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// mysqlSource serves MySQL and MariaDB with go-sql-driver/mysql, a pure Go driver. The server is a URL:
// mysql://user:password@host:3306/database?tls=true
func mysqlSource() Source {
	return sqlSource{open: openMySQL, readOnly: true}
}

func openMySQL(q Query) (*sql.DB, error) {
	cfg, err := mysqlConfig(q)
	if err != nil {
		return nil, err
	}
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("preparing the connection: %w", err)
	}
	return sql.OpenDB(connector), nil
}

// mysqlConfig reads the server URL. Only the options named here are accepted: an option the driver does not know as
// one of its own becomes a session variable, and a connection string must not be able to set those.
func mysqlConfig(q Query) (*mysql.Config, error) {
	u, err := url.Parse(q.Server)
	if err != nil || (u.Scheme != "mysql" && u.Scheme != "mariadb") || u.Host == "" {
		return nil, errors.New("the server must be a URL like mysql://user:password@host:3306/database")
	}
	cfg := mysql.NewConfig()
	cfg.User = u.User.Username()
	cfg.Passwd, _ = u.User.Password()
	cfg.Net, cfg.Addr = "tcp", withPort(u.Host)
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	cfg.Timeout = q.Timeout
	for name, values := range u.Query() {
		if name != "tls" {
			return nil, fmt.Errorf("the option %q is not supported: only tls (true, skip-verify, preferred or false) is", name)
		}
		cfg.TLSConfig = values[0]
	}
	return cfg, nil
}

func withPort(host string) string {
	if u, err := url.Parse("//" + host); err == nil && u.Port() != "" {
		return host
	}
	return host + ":3306"
}
