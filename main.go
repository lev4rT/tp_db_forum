package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
)

var DB *sql.DB

func simpleGet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You came and you get it!")
}
func main() {
	// Connect to postgreSql db
	DB, _ = sql.Open(
		"postgres",
		fmt.Sprint("user=postgres "+
			"password=admin "+
			"dbname=postgres "+
			"host=localhost "+
			"port=5432 "),
	)
	defer DB.Close()
	if err := DB.Ping(); err != nil {
		log.Fatal(err)
	}

	path := filepath.Join("script.sql")
	c, _ := ioutil.ReadFile(path)
	scriptString := string(c)
	DB.Exec(scriptString)

	r := mux.NewRouter()

	r.HandleFunc("/", simpleGet)
	r.HandleFunc("/get", simpleGet)
	r.HandleFunc("/api/service/clear", dbClearAll)
	r.HandleFunc("/api/user/{nickname}/create", createUser)
	r.HandleFunc("/api/forum/create", createForum)
	r.HandleFunc("/api/forum/{slug}/create", createThread)
	r.HandleFunc("/api/thread/{slug_or_id}/vote", voteThread)
	err := http.ListenAndServe(":5000", r)
	if err != nil {
		panic(err)
	}
}

func dbClearAll(w http.ResponseWriter, r *http.Request)  {
	path := filepath.Join("script.sql")
	c, _ := ioutil.ReadFile(path)
	scriptString := string(c)
	DB.Exec(scriptString)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

type User struct {
	Nickname string `json:"nickname"`
	Fullname string `json:"fullname"`
	About string `json:"about"`
	Email string `json:"email"`
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	vars := mux.Vars(r)
	user.Nickname = vars["nickname"]
	json.NewDecoder(r.Body).Decode(&user)

	_, err := DB.Exec(`INSERT INTO users(nickname, fullname, about, email) VALUES ($1, $2, $3, $4)`, user.Nickname, user.Fullname, user.About, user.Email)
	if err, ok := err.(*pq.Error); ok {
		switch err.Constraint {
		case "users_email_key":
			DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE email=$1`, user.Email).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(user); err != nil {
				panic(err)
			}
			return
		case "users_pkey":
			DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE nickname=$1`, user.Nickname).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(user); err != nil {
				panic(err)
			}
			return
		default:
			fmt.Println(err.Constraint)
			panic(err)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(user); err != nil {
		panic(err)
	}
}

type Forum struct {
	Title string `json:"title"`
	User string `json:"user"`
	Slug string `json:"slug"`
	Posts int64 `json:"posts"`
	Threads int64 `json:"threads"`
}

type ErrorMsg struct {
	Message string `json:"message"`
}

func createForum(w http.ResponseWriter, r *http.Request) {
	var forum Forum
	json.NewDecoder(r.Body).Decode(&forum)

	_, err := DB.Exec(`INSERT INTO forums(title, "user", slug) VALUES ($1, $2, $3)`, forum.Title, forum.User,  forum.Slug)
	if err, ok := err.(*pq.Error); ok {
		//fmt.Println("Severity:", err.Severity)
		//fmt.Println("Code:", err.Code)
		//fmt.Println("Message:", err.Message)
		//fmt.Println("Detail:", err.Detail)
		//fmt.Println("Hint:", err.Hint)
		//fmt.Println("Position:", err.Position)
		//fmt.Println("InternalPosition:", err.InternalPosition)
		//fmt.Println("Where:", err.Where)
		//fmt.Println("Schema:", err.Schema)
		//fmt.Println("Table:", err.Table)
		//fmt.Println("Column:", err.Column)
		//fmt.Println("DataTypeName:", err.DataTypeName)
		//fmt.Println("Constraint:", err.Constraint)
		//fmt.Println("File:", err.File)
		//fmt.Println("Line:", err.Line)
		//fmt.Println("Routine:", err.Routine)
		switch err.Constraint {
		case "forums_user_fkey":
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"Can't find user with nickname " + forum.User,
			}); err != nil {
				panic(err)
			}
			return
		case "forums_pkey":
			DB.QueryRow(`SELECT title, "user", slug, posts, threads FROM forums WHERE slug=$1`, forum.Slug).Scan(&forum.Title, &forum.User,  &forum.Slug, &forum.Posts, &forum.Threads)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(forum); err != nil {
				panic(err)
			}
			return
		default:
			fmt.Println(err.Constraint)
			panic(err)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(forum); err != nil {
		panic(err)
	}
}

type Thread struct {
	ID int `json:"id"`
	Title string `json:"title"`
	Author string `json:"author"`
	Forum string `json:"forum"`
	Message string `json:"message"`
	Votes int `json:"votes"`
	Slug string `json:"slug"`
	Created time.Time `json:"created"`
}

func createThread(w http.ResponseWriter, r *http.Request) {
	var thread Thread
	vars := mux.Vars(r)
	thread.Forum = vars["slug"]
	json.NewDecoder(r.Body).Decode(&thread)

	_, err := DB.Exec(`INSERT INTO threads(title, author, forum, message, votes, slug, created) VALUES ($1, $2, $3, $4, $5, $6, $7)`, thread.Title, thread.Author, thread.Forum, thread.Message, thread.Votes, thread.Slug, thread.Created)
	if err, ok := err.(*pq.Error); ok {
		//fmt.Println("Severity:", err.Severity)
		//fmt.Println("Code:", err.Code)
		//fmt.Println("Message:", err.Message)
		//fmt.Println("Detail:", err.Detail)
		//fmt.Println("Hint:", err.Hint)
		//fmt.Println("Position:", err.Position)
		//fmt.Println("InternalPosition:", err.InternalPosition)
		//fmt.Println("Where:", err.Where)
		//fmt.Println("Schema:", err.Schema)
		//fmt.Println("Table:", err.Table)
		//fmt.Println("Column:", err.Column)
		//fmt.Println("DataTypeName:", err.DataTypeName)
		//fmt.Println("Constraint:", err.Constraint)
		//fmt.Println("File:", err.File)
		//fmt.Println("Line:", err.Line)
		//fmt.Println("Routine:", err.Routine)
		switch err.Constraint {
		case "unique_thread":
			DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(thread); err != nil {
				panic(err)
			}
			return
		case "threads_forum_fkey":
		case "threads_author_fkey":
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"Author or forum slug doesnt exists",
			}); err != nil {
				panic(err)
			}
			return
		default:
			fmt.Println(err.Constraint)
			panic(err)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	if err := json.NewEncoder(w).Encode(thread); err != nil {
		panic(err)
	}
}

type Vote struct {
	Nickname string `json:"nickname"`
	Voice int `json:"voice"`
	ThreadID int `json:"thread_id"`
}

func voteThread (w http.ResponseWriter, r *http.Request) {
	var vote Vote
	vars := mux.Vars(r)
	vote.ThreadID, _ = strconv.Atoi(vars["slug_or_id"])
	json.NewDecoder(r.Body).Decode(&vote)

	_, err := DB.Exec(`INSERT INTO votes(nickname, voice, threadID) VALUES ($1, $2, $3)`, vote.Nickname, vote.Voice, vote.ThreadID)
	if err, ok := err.(*pq.Error); ok {
		fmt.Println("Severity:", err.Severity)
		fmt.Println("Code:", err.Code)
		fmt.Println("Message:", err.Message)
		fmt.Println("Detail:", err.Detail)
		fmt.Println("Hint:", err.Hint)
		fmt.Println("Position:", err.Position)
		fmt.Println("InternalPosition:", err.InternalPosition)
		fmt.Println("Where:", err.Where)
		fmt.Println("Schema:", err.Schema)
		fmt.Println("Table:", err.Table)
		fmt.Println("Column:", err.Column)
		fmt.Println("DataTypeName:", err.DataTypeName)
		fmt.Println("Constraint:", err.Constraint)
		fmt.Println("File:", err.File)
		fmt.Println("Line:", err.Line)
		fmt.Println("Routine:", err.Routine)
		switch err.Constraint {
		case "unique_vote":
			DB.Exec(`UPDATE votes SET voice = $1 WHERE "threadid" = $2 AND nickname = $3;`, vote.Voice, vote.ThreadID, vote.Nickname)
			break
		case "votes_threadID_fkey":
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"cant find thread!",
			}); err != nil {
				panic(err)
			}
			return
		default:
			fmt.Println(err.Constraint)
			panic(err)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	var thread Thread
	DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=$1`, vote.ThreadID).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	if err := json.NewEncoder(w).Encode(thread); err != nil {
		panic(err)
	}
}