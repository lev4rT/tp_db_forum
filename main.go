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
	"strings"
	"sync"
	"time"
)

var DB *sql.DB

func simpleGet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You came and you get it!")
}

//var threadIDForum map[int]string
//var threadIDMaxPostID map[int]int64
var threadIDForum sync.Map
//var threadIDMaxPostID map[int]int64

func main() {
	//threadIDForum = make(map[int]string)
	//threadIDMaxPostID = make(map[int]int64)

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

	r.HandleFunc("/api/service/clear", dbClearAll).Methods("POST")
	r.HandleFunc("/api/user/{nickname}/create", createUser).Methods("POST")
	r.HandleFunc("/api/forum/create", createForum).Methods("POST")
	r.HandleFunc("/api/forum/{slug}/create", createThread).Methods("POST")
	r.HandleFunc("/api/thread/{slug_or_id}/vote", voteThread).Methods("POST")
	r.HandleFunc("/api/thread/{slug_or_id}/create", createPost).Methods("POST")
	r.HandleFunc("/api/user/{nickname}/profile", getUserInfo).Methods("GET")
	r.HandleFunc("/api/user/{nickname}/profile", changeUserInfo).Methods("POST")
	r.HandleFunc("/api/forum/{slug}/details", getForumInfo).Methods("GET")
	r.HandleFunc("/api/forum/{slug}/threads", getThreadsInfo).Methods("GET")
	r.HandleFunc("/api/thread/{slug_or_id}/details", getThreadInfo).Methods("GET")
	r.HandleFunc("/api/thread/{slug_or_id}/posts", getThreadPosts).Methods("GET")
	r.HandleFunc("/api/thread/{slug_or_id}/details", changeThreadInfo).Methods("POST")
	r.HandleFunc("/api/forum/{slug}/users", getForumUsers).Methods("GET")

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
		fmt.Println("")
		fmt.Println("")
		fmt.Println("")
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
		var conflictUsers[] User
		var conflictUser1 User
		var conflictUser2 User
		DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE email=$1`, user.Email).Scan(&conflictUser1.Nickname, &conflictUser1.Fullname, &conflictUser1.About, &conflictUser1.Email)
		if conflictUser1.Nickname != "" {
			conflictUsers = append(conflictUsers, conflictUser1)
		}
		DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE nickname=$1`, user.Nickname).Scan(&conflictUser2.Nickname, &conflictUser2.Fullname, &conflictUser2.About, &conflictUser2.Email)
		if conflictUser2.Nickname != "" && conflictUser2 != conflictUser1{
			conflictUsers = append(conflictUsers, conflictUser2)
		}
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusConflict)
		if err := json.NewEncoder(w).Encode(conflictUsers); err != nil {
			panic(err)
		}
		return
		//switch err.Constraint {
		//case "users_email_key":
		//	fmt.Println(user)
		//	DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE email=$1`, user.Email).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
		//	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		//	w.WriteHeader(http.StatusConflict)
		//	fmt.Println(user)
		//	conflictUsers = append(conflictUsers, user)
		//	if err := json.NewEncoder(w).Encode(conflictUsers); err != nil {
		//		panic(err)
		//	}
		//	return
		//case "users_pkey":
		//	DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE nickname=$1`, user.Nickname).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
		//	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		//	w.WriteHeader(http.StatusConflict)
		//	var response[] User
		//	if err := json.NewEncoder(w).Encode(append(response, user)); err != nil {
		//		panic(err)
		//	}
		//	return
		//default:
		//	fmt.Println(err.Constraint)
		//	panic(err)
		//}
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

	res, err := DB.Query(`INSERT INTO forums(title, "user", slug) VALUES ($1, $2, $3) RETURNING title, "user", slug`, forum.Title, forum.User, forum.Slug)
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
			return
		}
	}

	if res != nil {
		defer res.Close()
	}
	DB.QueryRow(`SELECT nickname FROM users WHERE nickname=$1`, forum.User).Scan(&forum.User)
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
	if thread.Slug != "" {
		//fmt.Println(thread.Slug)
		DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE slug=$1`, thread.Slug).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
		if thread.ID != 0 {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(thread); err != nil {
				panic(err)
			}
			return
		}
	}

	_, err := DB.Exec(`INSERT INTO threads(title, author, forum, message, votes, slug, created) VALUES ($1, $2, $3, $4, $5, $6, $7)`, thread.Title, thread.Author, thread.Forum, thread.Message, thread.Votes, thread.Slug, thread.Created)
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
		case "unique_thread":
			DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(thread); err != nil {
				panic(err)
			}
			return
		case "threads_forum_fkey", "threads_author_fkey":
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"Author or forum slug doesnt exists",
			}); err != nil {
				panic(err)
			}
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	DB.QueryRow(fmt.Sprintf("SELECT slug FROM forums WHERE slug='%s'", thread.Forum)).Scan(&thread.Forum)

	//threadIDForum.Store(thread.ID, thread.Forum)
	//threadIDForum[thread.ID] = thread.Forum
	if err := json.NewEncoder(w).Encode(thread); err != nil {
		panic(err)
	}
}

type Vote struct {
	Nickname string `json:"nickname"`
	Voice int `json:"voice"`
	ThreadID int `json:"thread_id"`
}

const alpha = "abcdefghijklmnopqrstuvwxyz"
func voteThread (w http.ResponseWriter, r *http.Request) {
	var vote Vote
	json.NewDecoder(r.Body).Decode(&vote)
	vars := mux.Vars(r)
	slugOrId, _ := vars["slug_or_id"]
	if strings.ContainsAny(slugOrId, alpha) {
		DB.QueryRow(`SELECT id FROM threads WHERE slug=$1`, slugOrId).Scan(&vote.ThreadID)
	} else {
		vote.ThreadID, _ = strconv.Atoi(slugOrId)
	}

	_, err := DB.Exec(`INSERT INTO votes(nickname, voice, threadID) VALUES ($1, $2, $3)`, vote.Nickname, vote.Voice, vote.ThreadID)
	if err, ok := err.(*pq.Error); ok {
		//fmt.Println(vote.Voice, vote.ThreadID, vote.Nickname)
		//fmt.Println("Constraint:", err.Constraint)
		switch err.Constraint {
		case "unique_vote":
			DB.Exec(`UPDATE votes SET voice = $1 WHERE "threadid" = $2 AND nickname = $3;`, vote.Voice, vote.ThreadID, vote.Nickname)
			break
		case "votes_threadid_fkey":
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

type Post struct {
	ID int64 `json:"id"`
	Parent int64 `json:"parent"`
	Author string `json:"author"`
	Message string `json:"message"`
	IsEdited bool `json:"is_edited"`
	Forum string `json:"forum"`
	Thread int `json:"thread"`
	Created time.Time `json:"created"`
}

func createPost (w http.ResponseWriter, r *http.Request) {
	var posts[] Post
	vars := mux.Vars(r)
	threadSlugOrId, _ := vars["slug_or_id"]
	json.NewDecoder(r.Body).Decode(&posts)
	if len(posts) == 0 {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(posts); err != nil {
			panic(err)
		}
		return
	}

	// TODO: create a MAP to remove redundant SELECTs
	threadID, forumSlug := "", ""
	DB.QueryRow(`SELECT id, forum FROM threads WHERE slug=$1`, threadSlugOrId).Scan(&threadID, &forumSlug)
	if threadID == "" || forumSlug == "" {
		DB.QueryRow(`SELECT id, forum FROM threads WHERE id=$1`, threadSlugOrId).Scan(&threadID, &forumSlug)
	}
	if threadID == "" || forumSlug == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(ErrorMsg{
			"cant find thread!",
		}); err != nil {
			panic(err)
		}
		return
	}

	resultQueryValueString := ""
	// TODO: Validate PARENTS POST SOMEHOW!
	for index, _ := range posts {
		posts[index].Forum = forumSlug
		posts[index].Thread, _ = strconv.Atoi(threadID)
		resultQueryValueString += fmt.Sprintf("(%d, '%s', '%s', %d, '%s'),", posts[index].Parent, posts[index].Author, posts[index].Message, posts[index].Thread, posts[index].Forum)
	}
	resultQueryValueString = strings.TrimRight(resultQueryValueString, ",")

	res, err := DB.Query(fmt.Sprintf("INSERT INTO posts(parent, author, message, thread, forum) VALUES %s RETURNING id, created;", resultQueryValueString))
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
		case "posts_thread_fkey":
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"cant find thread!",
			}); err != nil {
				panic(err)
			}
			return
		}
	}

	defer res.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	for index, _ := range posts {
		res.Next()
		res.Scan(&posts[index].ID, &posts[index].Created)
	}
	if err := json.NewEncoder(w).Encode(posts); err != nil {
		panic(err)
	}
}

func getUserInfo (w http.ResponseWriter, r *http.Request) {
	var user User
	vars := mux.Vars(r)
	nickname, _ := vars["nickname"]

	DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
	if user.Nickname == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"Error",
		})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Println(user)
	json.NewEncoder(w).Encode(user)
}

func changeUserInfo (w http.ResponseWriter, r *http.Request) {
	var user User
	vars := mux.Vars(r)
	json.NewDecoder(r.Body).Decode(&user)
	nickname, _ := vars["nickname"]

	if user == (User{}) {
		err := DB.QueryRow(`SELECT fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Fullname, &user.About, &user.Email)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"User not found!",
			}); err != nil {
				panic(err)
			}
			return
		}
		user.Nickname = nickname
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Println(user)
		json.NewEncoder(w).Encode(user)
		return
	}
	setQuery := ""
	if user.Fullname != "" {
		setQuery += fmt.Sprintf("fullname='%s',", user.Fullname)
	}
	if user.About != "" {
		setQuery += fmt.Sprintf("about='%s',", user.About)
	}
	if user.Email != "" {
		setQuery += fmt.Sprintf("email='%s',", user.Email)
	}
	setQuery = strings.TrimRight(setQuery, ",") + " "

	user.Nickname = nickname
	res, err := DB.Query(fmt.Sprintf("UPDATE users SET %s  WHERE nickname='%s' RETURNING fullname, about, email", setQuery, user.Nickname))
	if err, ok := err.(*pq.Error); ok {
		switch err.Constraint {
		case "users_email_key", "users_nickname_key":
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"Conflict with other user!",
			}); err != nil {
				panic(err)
			}
			return
		}
	}


	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	if res == nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(ErrorMsg{
			"User not found!",
		}); err != nil {
			panic(err)
		}
		return
	}

	defer res.Close()
	res.Next()

	var updatedUser User
	res.Scan(&updatedUser.Fullname, &updatedUser.About, &updatedUser.Email)
	if updatedUser == (User{}) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"User not found!",
		})
		return
	}
	updatedUser.Nickname = nickname

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Println(user)
	json.NewEncoder(w).Encode(updatedUser)
}

func getForumInfo (w http.ResponseWriter, r *http.Request) {
	var forum Forum
	vars := mux.Vars(r)
	slug, _ := vars["slug"]

	DB.QueryRow(fmt.Sprintf("SELECT title, \"user\", slug, posts, threads FROM forums WHERE slug='%s'", slug)).Scan(&forum.Title, &forum.User, &forum.Slug, &forum.Posts, &forum.Threads)
	if (Forum{}) == forum {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum not found!",
		})
		return
	}

	DB.QueryRow(`SELECT nickname FROM users WHERE nickname=$1`, forum.User).Scan(&forum.User)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(forum)

}

func getThreadsInfo (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumSlug, _ := vars["slug"]

	err := DB.QueryRow(`SELECT slug FROM forums WHERE slug=$1`, forumSlug).Scan(&forumSlug)
	if err != nil || forumSlug == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	v := r.URL.Query()
	limit := v.Get("limit")
	since := v.Get("since")
	since = strings.Replace(since, "T", " ", -1)
	desc, _ := strconv.ParseBool(v.Get("desc"))

	query := fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum='%s' ", forumSlug)
	if since != "" {
		//query += "AND created >= '" + since + "' "
		query += "AND created"
		if desc {
			query += "<= '" + since + "' "
		} else {
			query += ">= '" + since + "' "
		}
	}

	query += "ORDER BY created "
	if desc {
		query += "DESC "
	} else {
		query += "ASC "
	}
	if limit == "" {
		limit = "100"
	}
	query += "LIMIT " + limit

	fmt.Println(query)
	res, _ := DB.Query(query)
	if res == nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	defer res.Close()

	threads := make([]Thread, 0)
	for res.Next() {
		var thread Thread
		res.Scan(&(thread.ID), &(thread.Title), &(thread.Author), &(thread.Forum), &(thread.Message), &(thread.Votes), &(thread.Slug), &(thread.Created))
		threads = append(threads, thread)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(threads)
}


func getThreadInfo (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	threadSlugOrId, _ := vars["slug_or_id"]

	var thread Thread
	DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE slug=$1`, threadSlugOrId).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	if thread.ID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=$1`, id).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	}

	if thread.Author == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"thread is not in system!",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(thread); err != nil {
		panic(err)
	}
}

func getThreadPosts (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	threadSlugOrId, _ := vars["slug_or_id"]
	threadID := 0

	DB.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
	if threadID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		DB.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE id=%d", id)).Scan(&threadID)
	}
	if threadID == 0 {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	v := r.URL.Query()
	limit := v.Get("limit")
	since := v.Get("since")
	since = strings.Replace(since, "T", " ", -1)
	sort := v.Get("sort")
	if sort == "" {
		sort = "flat"
	}
	desc, _ := strconv.ParseBool(v.Get("desc"))

	query := fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE thread=%d ", threadID)
	if since != "" && sort != "tree" {
		//query += "AND created >= '" + since + "' "
		query += "AND id"
		if desc {
			query += "<" + since + " "
		} else {
			query += ">" + since + " "
		}
	}

	if sort == "flat" {
		query += "ORDER BY id "
		if desc {
			query += "DESC "
		} else {
			query += "ASC "
		}
	} else if sort == "tree" {
		sign := ">"
		if desc {
			sign = "<"
		}
		if since != "" {
			query += fmt.Sprintf("AND path %s (SELECT path FROM posts WHERE id =%s)", sign, since)
		}
		query += "ORDER BY path "
		if desc {
			query += "DESC, "
		} else {
			query += "ASC, "
		}
		query += "id "
		if desc {
			query += "DESC "
		} else {
			query += "ASC "
		}
	}
	if limit == "" {
		limit = "100"
	}
	query += "LIMIT " + limit


	if sort == "parent_tree" {
		sign := ">"
		if desc {
			sign = "<"
		}
		sort := "ASC"
		if desc {
			sort = "DESC"
		}
		pathParam := "path"
		if desc {
			pathParam += "[1]"
		}
		if since == "" {
			query = fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE path[1] IN (SELECT id FROM posts WHERE thread = %d AND parent = 0 ORDER BY id %s LIMIT %s) ORDER BY ", threadID, sort, limit)
		} else {
			query = fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE path[1] IN (SELECT id FROM posts WHERE thread = %d AND parent = 0 AND path[1] %s (SELECT path[1] FROM posts WHERE id = %s) ORDER BY id %s LIMIT %s) ORDER BY ", threadID, sign, since, sort, limit)
		}
		if desc {
			query += fmt.Sprintf("path[1] DESC, path, id")
		} else {
			query += fmt.Sprintf("path, id")
		}
	}

	fmt.Println(query)
	res, _ := DB.Query(query)
	if res == nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	defer res.Close()

	posts := make([]Post, 0)
	for res.Next() {
		var post Post
		res.Scan(&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited, &post.Forum, &post.Thread, &post.Created)
		posts = append(posts, post)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(posts)
}

func changeThreadInfo (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	threadSlugOrId, _ := vars["slug_or_id"]
	threadID := 0
	var thread Thread
	json.NewDecoder(r.Body).Decode(&thread)

	DB.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
	if threadID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		DB.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE id=%d", id)).Scan(&threadID)
	}
	if threadID == 0 {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	setter := ""
	if thread.Title != "" {
		setter += "title= '" + thread.Title + "',"
	}
	if thread.Message != "" {
		setter += "message= '" + thread.Message + "',"
	}
	setter = strings.TrimRight(setter, ",")

	fmt.Printf("UPDATE threads SET %s WHERE id=%d", setter, threadID)
	DB.Exec(fmt.Sprintf("UPDATE threads SET %s WHERE id=%d", setter, threadID))
	fmt.Printf("SELECT title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", threadID)
	DB.QueryRow(fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", threadID)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(thread)
}

func getForumUsers (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumSlug, _ := vars["slug"]

	DB.QueryRow(fmt.Sprintf("SELECT slug FROM forum WHERE slug='%s'", forumSlug)).Scan(&forumSlug)
	if forumSlug == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	v := r.URL.Query()
	limit := v.Get("limit")
	since := v.Get("since")
	desc, _ := strconv.ParseBool(v.Get("desc"))
	query := fmt.Sprintf("SELECT nickname, fullname, about, email FROM usersonforums WHERE slug='%s' ", forumSlug)
	if since != "" {
		sign := ">"
		if desc {
			sign = "<"
		}
		query += fmt.Sprintf("AND nickname %s '%s' ", sign, since)
	}
	order := " ASC "
	if desc {
		order = " DESC "
	}
	query += fmt.Sprintf(" ORDER BY nickname %s ", order)

	if limit == "" {
		limit = "100"
	}
	query += fmt.Sprintf(" LIMIT %s ", limit)

	fmt.Println(query)
	res, _ := DB.Query(query)
	if res == nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}
	defer res.Close()

	users := make([]User, 0)
	for res.Next() {
		var user User
		res.Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
		users = append(users, user)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(users)
}