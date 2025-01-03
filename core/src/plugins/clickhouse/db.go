package clickhouse

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"github.com/clidey/whodb/core/src/log"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/clidey/whodb/core/src/common"
	"github.com/clidey/whodb/core/src/engine"
)

const (
	portKey         = "Port"
	sslModeKey      = "SSL Mode"
	httpProtocolKey = "HTTP Protocol"
	readOnlyKey     = "Readonly"
	debugKey        = "Debug"
)

func DB(config *engine.PluginConfig) (*sql.DB, error) {
	port := common.GetRecordValueOrDefault(config.Credentials.Advanced, portKey, "9000")
	sslMode := common.GetRecordValueOrDefault(config.Credentials.Advanced, sslModeKey, "disable")
	httpProtocol := common.GetRecordValueOrDefault(config.Credentials.Advanced, httpProtocolKey, "disable")
	readOnly := common.GetRecordValueOrDefault(config.Credentials.Advanced, readOnlyKey, "disable")
	debug := common.GetRecordValueOrDefault(config.Credentials.Advanced, debugKey, "disable")
	options := &clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", config.Credentials.Hostname, port)},
		Auth: clickhouse.Auth{
			Database: config.Credentials.Database,
			Username: config.Credentials.Username,
			Password: config.Credentials.Password,
		},
		DialTimeout:      time.Second * 30,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	}
	if debug != "disable" {
		options.Debug = true
	}
	if readOnly == "disable" {
		options.Settings = clickhouse.Settings{
			"max_execution_time": 60,
		}
	}
	if strings.HasPrefix(port, "8") || httpProtocol != "disable" {
		options.Protocol = clickhouse.HTTP
		options.Compression = &clickhouse.Compression{
			Method: clickhouse.CompressionGZIP,
		}
	} else {
		options.Compression = &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		}
		options.MaxOpenConns = 5
		options.MaxIdleConns = 5
		options.ConnMaxLifetime = time.Hour
	}
	if sslMode != "disable" {
		options.TLS = &tls.Config{InsecureSkipVerify: sslMode == "relaxed" || sslMode == "none"}
	}

	conn := clickhouse.OpenDB(options)
	err := conn.PingContext(context.Background())
	if err != nil {
		log.Logger.Warnf("clickhouse.Ping() error: %v", err)
	}
	return conn, err
}

func getTableColumns(conn driver.Conn, schema, table string) ([]engine.Record, error) {
	query := fmt.Sprintf("DESCRIBE TABLE %s.%s", schema, table)
	rows, err := conn.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []engine.Record
	for rows.Next() {
		var name, typ, defaultType, defaultExpression string
		var comment *string
		if err := rows.Scan(&name, &typ, &defaultType, &defaultExpression, &comment); err != nil {
			return nil, err
		}
		columns = append(columns, engine.Record{Key: name, Value: typ})
	}

	return columns, nil
}
