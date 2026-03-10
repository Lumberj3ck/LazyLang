package db

import (
	"database/sql"
	"fmt"

	petname "github.com/dustinkirkland/golang-petname"
)

func CreateChatSession(db *sql.DB) (int64, error) {
	queryTmpl := fmt.Sprintf(`INSERT into %s (name) VALUES (?);`, DefaultChatTableName)
	name := petname.Generate(2, "-")
	r, err := db.Exec(queryTmpl, name)
	if err != nil{
		return -1, err
	}
	n, err := r.LastInsertId()

	if err != nil{
		return -1, err
	}
	return n, nil
}
