// Package dbfiles embeds the SQL files that ship with the app.
package dbfiles

import _ "embed"

// Schema is the full database schema used to set up an empty database.
//
//go:embed schema.sql
var Schema string
