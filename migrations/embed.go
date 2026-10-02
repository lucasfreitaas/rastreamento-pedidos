// Package migrations embute os arquivos .sql no binário, para que a
// aplicação consiga migrar o banco sem depender de arquivos externos.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
