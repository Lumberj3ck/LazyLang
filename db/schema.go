package db

import (
	"fmt"

	"github.com/tmc/langchaingo/memory/sqlite3"
)

const DefaultChatTableName = "langchaingo_sessions"
const DefaultSchemaTmpl = `CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		name TEXT,
		session INTEGER,
		content TEXT NOT NULL,
		type TEXT NOT NULL,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY (session) REFERENCES %s(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_langchaingo_id ON %s (id);
CREATE INDEX IF NOT EXISTS idx_langchaingo_session ON %s (session);

CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		name TEXT,
		created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)`

var DefaultSchema = fmt.Sprintf(DefaultSchemaTmpl, sqlite3.DefaultTableName, DefaultChatTableName, sqlite3.DefaultTableName, sqlite3.DefaultTableName, DefaultChatTableName)
