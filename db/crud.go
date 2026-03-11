package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/charmbracelet/bubbles/list"
	petname "github.com/dustinkirkland/golang-petname"
	"github.com/tmc/langchaingo/memory/sqlite3"
)

type Chat struct{
	id int
	name string
	created time.Time
}
func (c Chat) Title() string       { return c.name }
func (c Chat) Description() string { return c.created.String() }
func (c Chat) FilterValue() string { return c.name }

var _ list.Item = Chat{}

func CreateChatSession(db *sql.DB) (int64, error) {
	queryTmpl := fmt.Sprintf(`INSERT into %s (name) VALUES (?);`, DefaultChatTableName)
	name := petname.Generate(2, "-")
	r, err := db.Exec(queryTmpl, name)
	if err != nil {
		return -1, err
	}
	n, err := r.LastInsertId()

	if err != nil {
		return -1, err
	}
	return n, nil
}

func GetActiveChats(db *sql.DB) ([]Chat, error) {
	query := fmt.Sprintf(`
	select DISTINCT ls.id, ls.name, ls.created from %s ls 
	JOIN %s lm 
	On ls.id = lm.session;`, DefaultChatTableName, sqlite3.DefaultTableName)
	res, err := db.Query(query)
	if err != nil{
		return nil, err
	}

	defer res.Close()
	var chats []Chat
	for res.Next() {
		var name string
		var id int
		var created string

		if err = res.Scan(&id, &name, &created); err != nil {
			return nil, err
		}
		cr, err := time.Parse("2006-01-02T15:04:05Z", created)
		if err != nil{
			log.Printf("Couldn't parse date of session with id %d: %v", id, err)
			continue
		}
		chats = append(chats, Chat{id, name, cr})
	}
	return chats, nil
}
