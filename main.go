package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)


func simpleGet(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "You came and you get it!")
}

var DB *pgx.ConnPool

func main() {
	config, err := pgx.ParseConnectionString(fmt.Sprintf("user=%s password=%s dbname=%s port=%s", "docker", "docker", "docker", "5432"))
	if err != nil {
		panic(err)
	}

	config.PreferSimpleProtocol = true
	connPoolConfig := pgx.ConnPoolConfig { ConnConfig: config, MaxConnections: 50, AcquireTimeout: 0, AfterConnect: nil }

	DB, err = pgx.NewConnPool(connPoolConfig)
	if err != nil {
		panic(err)
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
	r.HandleFunc("/api/post/{id}/details", getPostInfo).Methods("GET")
	r.HandleFunc("/api/post/{id}/details", changePostMessage).Methods("POST")
	r.HandleFunc("/api/service/status", getServiceStatus).Methods("GET")

	err = http.ListenAndServe(":5000", r)
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
	if err != nil  {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			res, _ := DB.Query(`SELECT nickname, fullname, about, email FROM users WHERE email=$1 or nickname=$2`, user.Email, user.Nickname)
			defer res.Close()

			users := make([]User, 0)
			for res.Next() {
				var user User
				res.Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
				users = append(users, user)
			}
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(users)
			return

			//w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			//w.WriteHeader(http.StatusOK)
			//json.NewEncoder(w).Encode(users)
			//
			//
			//var conflictUsers[] User
			//var conflictUser User
			//DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE email=$1 or nickname=$2`, user.Email, user.Nickname).Scan(&conflictUser.Nickname, &conflictUser.Fullname, &conflictUser.About, &conflictUser.Email)
			//w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			//w.WriteHeader(http.StatusConflict)
			//conflictUsers = append(conflictUsers, conflictUser)
			//if err := json.NewEncoder(w).Encode(conflictUsers); err != nil {
			//	panic(err)
			//}
			//return
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

	_, err := DB.Exec(`INSERT INTO forums(title, "user", slug) VALUES ($1, $2, $3)`, forum.Title, forum.User, forum.Slug)

	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			DB.QueryRow(`SELECT title, "user", slug, posts, threads FROM forums WHERE slug=$1`, forum.Slug).Scan(&forum.Title, &forum.User,  &forum.Slug, &forum.Posts, &forum.Threads)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(forum); err != nil {
				panic(err)
			}
			return
		}
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23503" {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"Can't find user with nickname " + forum.User,
			}); err != nil {
				panic(err)
			}
			return
		}
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
	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23503" {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"Author or forum slug doesnt exists",
			}); err != nil {
				panic(err)
			}
			return
		}

		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(thread); err != nil {
				panic(err)
			}
			return
		}
	}
	//if err, ok := err.(*pq.Error); ok {
	//	switch err.Constraint {
	//	case "unique_thread":
	//		DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	//		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//		w.WriteHeader(http.StatusConflict)
	//		if err := json.NewEncoder(w).Encode(thread); err != nil {
	//			panic(err)
	//		}
	//		return
	//	case "threads_forum_fkey", "threads_author_fkey":
	//		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//		w.WriteHeader(http.StatusNotFound)
	//		if err := json.NewEncoder(w).Encode(ErrorMsg{
	//			"Author or forum slug doesnt exists",
	//		}); err != nil {
	//			panic(err)
	//		}
	//		return
	//	}
	//}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	DB.QueryRow(fmt.Sprintf("SELECT slug FROM forums WHERE slug='%s'", thread.Forum)).Scan(&thread.Forum)

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

	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23503" {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"cant find thread!",
			}); err != nil {
				panic(err)
			}
			return
		}

		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			DB.Exec(`UPDATE votes SET voice = $1 WHERE "threadid" = $2 AND nickname = $3;`, vote.Voice, vote.ThreadID, vote.Nickname)
		}
	}

	//if err, ok := err.(*pq.Error); ok {
	//	switch err.Constraint {
	//	case "uniquevote":
	//		DB.Exec(`UPDATE votes SET voice = $1 WHERE "threadid" = $2 AND nickname = $3;`, vote.Voice, vote.ThreadID, vote.Nickname)
	//		break
	//	case "votes_threadid_fkey", "posts_author_fkey", "votesnicknamethreadid", "votes_nickname_fkey":
	//		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//		w.WriteHeader(http.StatusNotFound)
	//		if err := json.NewEncoder(w).Encode(ErrorMsg{
	//			"cant find thread!",
	//		}); err != nil {
	//			panic(err)
	//		}
	//		return
	//	default:
	//		panic(err)
	//	}
	//}

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
	IsEdited bool `json:"isEdited"`
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
		threadID := 0
		DB.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE id=%s", threadSlugOrId)).Scan(&threadID)
		if threadID == 0 {
			DB.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
		}
		if threadID == 0 {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"cant find thread!",
			}); err != nil {
				panic(err)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(posts); err != nil {
			panic(err)
		}
		return
	}

	// TODO: create a MAP to remove redundant SELECTs
	threadID, forumSlug := -1, ""
	DB.QueryRow(`SELECT id, forum FROM threads WHERE slug=$1`, threadSlugOrId).Scan(&threadID, &forumSlug)
	if threadID == -1 || forumSlug == "" {
		DB.QueryRow(`SELECT id, forum FROM threads WHERE id=$1`, threadSlugOrId).Scan(&threadID, &forumSlug)
	}
	if threadID == -1 || forumSlug == "" {
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
		posts[index].Thread = threadID

		if posts[index].Parent != 0 {
			thread := 0
			DB.QueryRow(fmt.Sprintf("SELECT thread FROM posts WHERE id=%d", posts[index].Parent)).Scan(&thread)
			if thread != posts[index].Thread {
				w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(ErrorMsg{
					"Parent post was created in another thread!",
				})
				return
			}
		}
		resultQueryValueString += fmt.Sprintf("(%d, '%s', '%s', %d, '%s'),", posts[index].Parent, posts[index].Author, posts[index].Message, posts[index].Thread, posts[index].Forum)
	}
	resultQueryValueString = strings.TrimRight(resultQueryValueString, ",")

	//fmt.Printf("\nINSERT INTO posts(parent, author, message, thread, forum) VALUES %s RETURNING id, created;", resultQueryValueString)
	res, _ := DB.Query(fmt.Sprintf("INSERT INTO posts(parent, author, message, thread, forum) VALUES %s RETURNING id, created;", resultQueryValueString))
	//if err, ok := err.(*pq.Error); ok {
	//	switch err.Constraint {
	//	case "posts_thread_fkey":
	//		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//		w.WriteHeader(http.StatusNotFound)
	//		if err := json.NewEncoder(w).Encode(ErrorMsg{
	//			"cant find thread!",
	//		}); err != nil {
	//			panic(err)
	//		}
	//		return
	//	case "posts_author_fkey":
	//		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//		w.WriteHeader(http.StatusNotFound)
	//		if err := json.NewEncoder(w).Encode(ErrorMsg{
	//			"NO author!",
	//		}); err != nil {
	//			panic(err)
	//		}
	//		return
	//	}
	//}

	defer res.Close()
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	for index, _ := range posts {
		res.Next()
		err := res.Scan(&posts[index].ID, &posts[index].Created)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			if err := json.NewEncoder(w).Encode(ErrorMsg{
				"cant find something!",
			}); err != nil {
				panic(err)
			}
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
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
	json.NewEncoder(w).Encode(user)
}

func changeUserInfo (w http.ResponseWriter, r *http.Request) {
	var user User
	vars := mux.Vars(r)
	json.NewDecoder(r.Body).Decode(&user)
	nickname, _ := vars["nickname"]

	if user == (User{}) {

		/* err := */ DB.QueryRow(`SELECT fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Fullname, &user.About, &user.Email)
		//if _, ok := err.(*pq.Error); ok {
		//	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		//	w.WriteHeader(http.StatusNotFound)
		//	if err := json.NewEncoder(w).Encode(ErrorMsg{
		//		"User not found!",
		//	}); err != nil {
		//		panic(err)
		//	}
		//	return
		//}
		user.Nickname = nickname
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
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
	//fmt.Printf("UPDATE users SET %s  WHERE nickname='%s' RETURNING fullname, about, email", setQuery, user.Nickname)
	var updatedUser User
	err := DB.QueryRow(fmt.Sprintf("UPDATE users SET %s  WHERE nickname='%s' RETURNING fullname, about, email", setQuery, user.Nickname)).Scan(&updatedUser.Fullname, &updatedUser.About, &updatedUser.Email)
	if err != nil {
		errPg, _ := err.(pgx.PgError)
		//fmt.Println("\n\n", errPg.Code, errPg.Message)
		if errPg.Code == "23505" {
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
	if updatedUser == (User{}) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(ErrorMsg{
			"User not found!",
		}); err != nil {
			panic(err)
		}
		return
	}
	updatedUser.Nickname = nickname


	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//if res == nil {
	//	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//	w.WriteHeader(http.StatusNotFound)
	//	if err := json.NewEncoder(w).Encode(ErrorMsg{
	//		"User not found!",
	//	}); err != nil {
	//		panic(err)
	//	}
	//	return
	//}
	//
	//defer res.Close()
	//res.Next()
	//
	//var updatedUser User
	//res.Scan(&updatedUser.Fullname, &updatedUser.About, &updatedUser.Email)
	//if updatedUser == (User{}) {
	//	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	//	w.WriteHeader(http.StatusNotFound)
	//	json.NewEncoder(w).Encode(ErrorMsg{
	//		"User not found!",
	//	})
	//	return
	//}
	updatedUser.Nickname = nickname

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
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
	v := r.URL.Query()
	limitParam := v.Get("limit")
	if limitParam == "" {
		limitParam = "100"
	}
	sinceParam := v.Get("since")
	sinceParam = strings.Replace(sinceParam, "T", " ", -1)
	sortParam := v.Get("sort")
	descParam, _ := strconv.ParseBool(v.Get("desc"))
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

	sortOrder := "ASC"
	if descParam {
		sortOrder = "DESC"
	}
	compare := ">"
	if descParam {
		compare = "<"
	}

	query := "SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE "
	if sortParam == "" || sortParam == "flat" {
		since := ""
		if sinceParam != "" {
			since = "AND id" + compare + sinceParam
		}
		query += fmt.Sprintf("thread=%d %s ORDER BY id %s LIMIT %s", threadID, since, sortOrder, limitParam)
	} else if sortParam == "tree" {
		since := ""
		if sinceParam != "" {
			since = "AND path " + compare + "( SELECT path FROM posts WHERE id = " + sinceParam + ")"
		}
		query += fmt.Sprintf("thread=%d %s ORDER BY path %s, id %s LIMIT %s", threadID, since, sortOrder, sortOrder, limitParam)
	} else if sortParam == "parent_tree" {
		since := ""
		if sinceParam != "" {
			since = fmt.Sprintf("AND path[1] %s (SELECT path[1] FROM posts WHERE id=%s)", compare, sinceParam)
		}
		query += fmt.Sprintf("path[1] IN (SELECT id FROM posts WHERE thread=%d AND parent=0 %s ORDER BY id %s LIMIT %s) ", threadID, since, sortOrder, limitParam)
		if descParam {
			query += "ORDER BY path[1] DESC, path, id"
		} else {
			query += "ORDER BY path, id"
		}
	}

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

	//fmt.Printf("UPDATE threads SET %s WHERE id=%d", setter, threadID)
	DB.Exec(fmt.Sprintf("UPDATE threads SET %s WHERE id=%d", setter, threadID))
	//fmt.Printf("SELECT title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", threadID)
	DB.QueryRow(fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", threadID)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(thread)
}

func getForumUsers (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	forumSlug, _ := vars["slug"]

	slug := ""
	DB.QueryRow(fmt.Sprintf("SELECT slug FROM forums WHERE slug='%s'", forumSlug)).Scan(&slug)
	if slug == "" {
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
		query += fmt.Sprintf("AND nickname %s '%s' COLLATE \"C\" ", sign, since)
	}
	order := " ASC "
	if desc {
		order = " DESC "
	}
	query += fmt.Sprintf(" ORDER BY nickname COLLATE \"C\" %s ", order)

	if limit == "" {
		limit = "100"
	}
	query += fmt.Sprintf(" LIMIT %s ", limit)

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

func getPostInfo (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postID, _ := strconv.Atoi(vars["id"])

	var post Post
	DB.QueryRow(fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE id=%d", postID)).Scan(&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited, &post.Forum, &post.Thread, &post.Created)
	if post.ID == 0 {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	jsonAnswer := make(map[string]interface{}, 0)
	jsonAnswer["post"] = post

	v := r.URL.Query()
	related := v.Get("related")

	if strings.Contains(related, "user") {
		var user User
		DB.QueryRow(fmt.Sprintf("SELECT nickname, fullname, about, email FROM users WHERE nickname='%s'", post.Author)).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
		jsonAnswer["author"] = user
	}
	if strings.Contains(related, "thread") {
		var thread Thread
		DB.QueryRow(fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", post.Thread)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
		jsonAnswer["thread"] = thread
	}
	if strings.Contains(related, "forum") {
		var forum Forum
		DB.QueryRow(fmt.Sprintf("SELECT title, \"user\", slug, posts, threads FROM forums WHERE slug='%s'", post.Forum)).Scan(&forum.Title, &forum.User, &forum.Slug, &forum.Posts, &forum.Threads)
		jsonAnswer["forum"] = forum
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(jsonAnswer)
}

func changePostMessage (w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	postID, _ := strconv.Atoi(vars["id"])
	var newPost Post
	json.NewDecoder(r.Body).Decode(&newPost)

	setQuery := ""
	if newPost.Message != "" {
		setQuery += fmt.Sprintf(" message = '%s'", newPost.Message)
	}
	if newPost.Author != "" {
		setQuery += fmt.Sprintf(" author = '%s',", newPost.Author)
	}
	if newPost.Parent != 0 {
		thread := 0
		DB.QueryRow(fmt.Sprintf("SELECT thread FROM posts WHERE id=%d", newPost.Parent)).Scan(&thread)
		if thread == newPost.Thread {
			setQuery += fmt.Sprintf(" parent = %d,", newPost.Parent)
		} else {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(ErrorMsg{
				"Parent post was created in another thread!",
			})
			return
		}
	}
	setQuery = strings.TrimRight(setQuery, ",") + " "

	DB.QueryRow(fmt.Sprintf("UPDATE posts SET %s WHERE id = %d", setQuery, postID))

	DB.QueryRow(fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE id=%d", postID)).Scan(&newPost.ID, &newPost.Parent, &newPost.Author, &newPost.Message, &newPost.IsEdited, &newPost.Forum, &newPost.Thread, &newPost.Created)
	if newPost.ID == 0 {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorMsg{
			"post is not in system!",
		})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newPost)
}

type Service struct {
	User int64 `json:"user"`
	Forum int64 `json:"forum"`
	Thread int64 `json:"thread"`
	Post int64 `json:"post"`

}
func getServiceStatus  (w http.ResponseWriter, r *http.Request) {
	var service Service

	DB.QueryRow("SELECT COUNT(*) FROM forums").Scan(&service.Forum)
	DB.QueryRow("SELECT COUNT(*) FROM threads").Scan(&service.Thread)
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&service.User)
	DB.QueryRow("SELECT COUNT(*) FROM posts").Scan(&service.Post)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(service)
}