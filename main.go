package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fasthttp/router"
	"github.com/jackc/pgconn"
	_ "github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/valyala/fasthttp"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var DB *pgxpool.Pool

func main() {
	urlExample := "postgres://docker:docker@localhost:5432/docker"
	DB,_ = pgxpool.Connect(context.Background(), urlExample)

	path := filepath.Join("script.sql")
	c, _ := ioutil.ReadFile(path)
	scriptString := string(c)
	DB.Exec(context.Background(), scriptString)


	r := router.New()
	r.POST("/api/service/clear", dbClearAll)
	r.POST("/api/user/{nickname}/create", createUser)
	r.POST("/api/forum/create", createForum)
	r.POST("/api/forum/{slug}/create", createThread)
	r.POST("/api/thread/{slug_or_id}/vote", voteThread)
	r.POST("/api/thread/{slug_or_id}/create", createPost)
	r.GET("/api/user/{nickname}/profile", getUserInfo)
	r.POST("/api/user/{nickname}/profile", changeUserInfo)
	r.GET("/api/forum/{slug}/details", getForumInfo)
	r.GET("/api/forum/{slug}/threads", getThreadsInfo)
	r.GET("/api/thread/{slug_or_id}/details", getThreadInfo)
	r.GET("/api/thread/{slug_or_id}/posts", getThreadPosts)
	r.POST("/api/thread/{slug_or_id}/details", changeThreadInfo)
	r.GET("/api/forum/{slug}/users", getForumUsers)
	r.GET("/api/post/{id}/details", getPostInfo)
	r.POST("/api/post/{id}/details", changePostMessage)
	r.GET("/api/service/status", getServiceStatus)

	log.Fatal(fasthttp.ListenAndServe(":5000", r.Handler))

	//r := mux.NewRouter()

	//r.HandleFunc("/api/service/clear", dbClearAll).Methods("POST")
	//r.HandleFunc("/api/user/{nickname}/create", createUser).Methods("POST")
	//r.HandleFunc("/api/forum/create", createForum).Methods("POST")
	//r.HandleFunc("/api/forum/{slug}/create", createThread).Methods("POST")
	//r.HandleFunc("/api/thread/{slug_or_id}/vote", voteThread).Methods("POST")
	//r.HandleFunc("/api/thread/{slug_or_id}/create", createPost).Methods("POST")
	//r.HandleFunc("/api/user/{nickname}/profile", getUserInfo).Methods("GET")
	//r.HandleFunc("/api/user/{nickname}/profile", changeUserInfo).Methods("POST")
	//r.HandleFunc("/api/forum/{slug}/details", getForumInfo).Methods("GET")
	//r.HandleFunc("/api/forum/{slug}/threads", getThreadsInfo).Methods("GET")
	//r.HandleFunc("/api/thread/{slug_or_id}/details", getThreadInfo).Methods("GET")
	//r.HandleFunc("/api/thread/{slug_or_id}/posts", getThreadPosts).Methods("GET")
	//r.HandleFunc("/api/thread/{slug_or_id}/details", changeThreadInfo).Methods("POST")
	//r.HandleFunc("/api/forum/{slug}/users", getForumUsers).Methods("GET")
	//r.HandleFunc("/api/post/{id}/details", getPostInfo).Methods("GET")
	//r.HandleFunc("/api/post/{id}/details", changePostMessage).Methods("POST")
	//r.HandleFunc("/api/service/status", getServiceStatus).Methods("GET")

	//err := http.ListenAndServe(":5000", r)
	//if err != nil {
	//	panic(err)
	//}
}

var (
	strContentType = []byte("Content-Type")
	strApplicationJSON = []byte("application/json")
)

func dbClearAll(ctx *fasthttp.RequestCtx)  {
	path := filepath.Join("script.sql")
	c, _ := ioutil.ReadFile(path)
	scriptString := string(c)
	DB.Exec(context.Background(), scriptString)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
}

type User struct {
	Nickname string `json:"nickname"`
	Fullname string `json:"fullname"`
	About string `json:"about"`
	Email string `json:"email"`
}

func createUser(ctx *fasthttp.RequestCtx) {
	var user User
	user.Nickname = ctx.UserValue("nickname").(string)
	json.Unmarshal(ctx.PostBody(), &user)

	_, err := DB.Exec(context.Background(), `INSERT INTO users(nickname, fullname, about, email) VALUES ($1, $2, $3, $4)`, user.Nickname, user.Fullname, user.About, user.Email)

	if err != nil  {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				res, _ := DB.Query(context.Background(), fmt.Sprintf("SELECT nickname, fullname, about, email FROM users WHERE email='%s' or nickname='%s'", user.Email, user.Nickname))
				defer res.Close()

				users := make([]User, 0)
				for res.Next() {
					var user User
					res.Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
					users = append(users, user)
				}
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusConflict)
				json.NewEncoder(ctx).Encode(users)
				return
			}
		}
	}

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(user)
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

func createForum(ctx *fasthttp.RequestCtx) {
	var forum Forum
	json.Unmarshal(ctx.PostBody(), &forum)

	_, err := DB.Exec(context.Background(), `INSERT INTO forums(title, "user", slug) VALUES ($1, $2, $3)`, forum.Title, forum.User, forum.Slug)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				DB.QueryRow(context.Background(), `SELECT title, "user", slug, posts, threads FROM forums WHERE slug=$1`, forum.Slug).Scan(&forum.Title, &forum.User,  &forum.Slug, &forum.Posts, &forum.Threads)
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusConflict)
				json.NewEncoder(ctx).Encode(forum)
				return
				//w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				//w.WriteHeader(http.StatusConflict)
				//if err := json.NewEncoder(w).Encode(forum); err != nil {
				//	panic(err)
				//}
				//return
			} else if pgErr.Code == "23503" {
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusNotFound)
				json.NewEncoder(ctx).Encode(forum)
				return
				//w.Header().Set("Content-Type", "application/json; charset=UTF-8")
				//w.WriteHeader(http.StatusNotFound)
				//if err := json.NewEncoder(w).Encode(ErrorMsg{
				//	"Can't find user with nickname " + forum.User,
				//}); err != nil {
				//	panic(err)
				//}
				//return
			}
		}
	}


	DB.QueryRow(context.Background(), `SELECT nickname FROM users WHERE nickname=$1`, forum.User).Scan(&forum.User)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(forum)
	return
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

func createThread(ctx *fasthttp.RequestCtx) {
	var thread Thread
	thread.Forum = ctx.UserValue("slug").(string)
	json.Unmarshal(ctx.PostBody(), &thread)

	if thread.Slug != "" {
		DB.QueryRow(context.Background(), `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE slug=$1`, thread.Slug).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
		if thread.ID != 0 {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(thread)
			return
		}
	}

	_, err := DB.Exec(context.Background(), `INSERT INTO threads(title, author, forum, message, votes, slug, created) VALUES ($1, $2, $3, $4, $5, $6, $7)`, thread.Title, thread.Author, thread.Forum, thread.Message, thread.Votes, thread.Slug, thread.Created)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" {
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusNotFound)
				json.NewEncoder(ctx).Encode(ErrorMsg{
					"Author or forum slug doesnt exists",
				})
				return
			} else if pgErr.Code == "23505" {
				DB.QueryRow(context.Background(), `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusConflict)
				json.NewEncoder(ctx).Encode(thread)
				return
			}
		}
	}

	DB.QueryRow(context.Background(), `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT slug FROM forums WHERE slug='%s'", thread.Forum)).Scan(&thread.Forum)

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(thread)
	return
}

type Vote struct {
	Nickname string `json:"nickname"`
	Voice int `json:"voice"`
	ThreadID int `json:"thread_id"`
}

const alpha = "abcdefghijklmnopqrstuvwxyz"
func voteThread (ctx *fasthttp.RequestCtx) {
	var vote Vote
	slugOrId := ctx.UserValue("slug_or_id").(string)
	json.Unmarshal(ctx.PostBody(), &vote)

	if strings.ContainsAny(slugOrId, alpha) {
		DB.QueryRow(context.Background(), `SELECT id FROM threads WHERE slug=$1`, slugOrId).Scan(&vote.ThreadID)
	} else {
		vote.ThreadID, _ = strconv.Atoi(slugOrId)
	}

	_, err := DB.Exec(context.Background(), `INSERT INTO votes(nickname, voice, threadID) VALUES ($1, $2, $3)`, vote.Nickname, vote.Voice, vote.ThreadID)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23503" {
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusNotFound)
				json.NewEncoder(ctx).Encode(ErrorMsg{
					"cant find thread!",
				})
				return
			} else if pgErr.Code == "23505" {
				DB.Exec(context.Background(), `UPDATE votes SET voice = $1 WHERE "threadid" = $2 AND nickname = $3;`, vote.Voice, vote.ThreadID, vote.Nickname)
			}
		}
	}

	var thread Thread
	DB.QueryRow(context.Background(), `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=$1`, vote.ThreadID).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(thread)
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

func createPost (ctx *fasthttp.RequestCtx) {
	var posts[] Post
	threadSlugOrId := ctx.UserValue("slug_or_id").(string)
	threadSlugOrIdConverted, _ := strconv.Atoi(threadSlugOrId)
	json.Unmarshal(ctx.PostBody(), &posts)
	threadID, forumSlug := -1, ""
	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id, forum FROM threads WHERE slug='%s' or id=%d", threadSlugOrId, threadSlugOrIdConverted)).Scan(&threadID, &forumSlug)
	if threadID == -1 || forumSlug == "" {

		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"cant find thread!",
		})
		return
	}
	if len(posts) == 0 {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusCreated)
		json.NewEncoder(ctx).Encode(posts)
		return
	}


	if posts[0].Parent != 0 {
		var pThread int
		err := DB.QueryRow(context.Background(),
			"SELECT thread FROM posts WHERE id = $1",
			posts[0].Parent,
		).Scan(&pThread)

		if err != nil {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(ErrorMsg{
				"cant find thread!",
			})
			return
		}

		if pThread != threadID {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(ErrorMsg{
				"Parent post was created in another thread!",
			})
			return
		}
	}


	resultQueryValueString := ""
	// TODO: Validate PARENTS POST SOMEHOW!
	for index, _ := range posts {
		posts[index].Forum = forumSlug
		posts[index].Thread = threadID

		//if posts[index].Parent != 0 {
		//	thread := 0
		//	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT thread FROM posts WHERE id=%d", posts[index].Parent)).Scan(&thread)
		//	if thread != posts[index].Thread {
		//		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		//		w.WriteHeader(http.StatusConflict)
		//		json.NewEncoder(w).Encode(ErrorMsg{
		//			"Parent post was created in another thread!",
		//		})
		//		return
		//	}
		//}
		resultQueryValueString += fmt.Sprintf("(%d, '%s', '%s', %d, '%s'),", posts[index].Parent, posts[index].Author, posts[index].Message, posts[index].Thread, posts[index].Forum)
	}
	resultQueryValueString = strings.TrimRight(resultQueryValueString, ",")

	res, _ := DB.Query(context.Background(), fmt.Sprintf("INSERT INTO posts(parent, author, message, thread, forum) VALUES %s RETURNING id, created;", resultQueryValueString))

	defer res.Close()
	var err error
	for index, _ := range posts {
		res.Next()
		err = res.Scan(&posts[index].ID, &posts[index].Created)
	}
	if err != nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"cant find something!",
		})
		return
	}
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(posts)
	return
}

func getUserInfo (ctx *fasthttp.RequestCtx) {
	var user User
	nickname := ctx.UserValue("nickname").(string)

	DB.QueryRow(context.Background(), `SELECT nickname, fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
	if user.Nickname == "" {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{"User not found"})
		return
	}
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(user)
}

func changeUserInfo (ctx *fasthttp.RequestCtx) {
	var user User
	nickname := ctx.UserValue("nickname").(string)
	json.Unmarshal(ctx.PostBody(), &user)

	if user == (User{}) {
		DB.QueryRow(context.Background(), `SELECT fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Fullname, &user.About, &user.Email)
		user.Nickname = nickname

		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusOK)
		json.NewEncoder(ctx).Encode(user)
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
	err := DB.QueryRow(context.Background(), fmt.Sprintf("UPDATE users SET %s  WHERE nickname='%s' RETURNING fullname, about, email", setQuery, user.Nickname)).Scan(&updatedUser.Fullname, &updatedUser.About, &updatedUser.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
				ctx.Response.SetStatusCode(http.StatusConflict)
				json.NewEncoder(ctx).Encode(ErrorMsg{"Conflict with other user"})
				return
			}
		}
	}
	if updatedUser == (User{}) {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{"User not found"})
		return
	}
	updatedUser.Nickname = nickname


	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(updatedUser)
	return
}

func getForumInfo (ctx *fasthttp.RequestCtx) {
	var forum Forum
	slug := ctx.UserValue("slug").(string)

	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT title, \"user\", slug, posts, threads FROM forums WHERE slug='%s'", slug)).Scan(&forum.Title, &forum.User, &forum.Slug, &forum.Posts, &forum.Threads)
	if (Forum{}) == forum {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"forum not found!",
		})
		return
	}

	DB.QueryRow(context.Background(), `SELECT nickname FROM users WHERE nickname=$1`, forum.User).Scan(&forum.User)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(forum)
	return

}

func getThreadsInfo (ctx *fasthttp.RequestCtx) {
	forumSlug := ctx.UserValue("slug").(string)

	err := DB.QueryRow(context.Background(), `SELECT slug FROM forums WHERE slug=$1`, forumSlug).Scan(&forumSlug)
	if err != nil || forumSlug == "" {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	limit := string(ctx.QueryArgs().Peek("limit"))
	since := string(ctx.QueryArgs().Peek("since"))
	since = strings.Replace(since, "T", " ", -1)
	desc, _ := strconv.ParseBool(string(ctx.QueryArgs().Peek("desc")))

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

	res, _ := DB.Query(context.Background(), query)
	if res == nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
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

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(threads)
	return
}


func getThreadInfo (ctx *fasthttp.RequestCtx) {
	threadSlugOrId := ctx.UserValue("slug_or_id").(string)

	var thread Thread
	DB.QueryRow(context.Background(), `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE slug=$1`, threadSlugOrId).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	if thread.ID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		DB.QueryRow(context.Background(), `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=$1`, id).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	}

	if thread.Author == "" {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"thread is not in system!",
		})
		return
	}

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(thread)
	return
}

func getThreadPosts (ctx *fasthttp.RequestCtx) {
	threadSlugOrId := ctx.UserValue("slug_or_id").(string)
	limitParam := string(ctx.QueryArgs().Peek("limit"))
	if limitParam == "" {
		limitParam = "100"
	}
	sinceParam := string(ctx.QueryArgs().Peek("since"))
	sinceParam = strings.Replace(sinceParam, "T", " ", -1)
	sortParam := string(ctx.QueryArgs().Peek("sort"))
	descParam, _ := strconv.ParseBool(string(ctx.QueryArgs().Peek("desc")))
	threadID := 0

	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
	if threadID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id FROM threads WHERE id=%d", id)).Scan(&threadID)
	}
	if threadID == 0 {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
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

	res, _ := DB.Query(context.Background(), query)
	if res == nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
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

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(posts)
	return
}

func changeThreadInfo (ctx *fasthttp.RequestCtx) {
	threadSlugOrId, _ := ctx.UserValue("slug_or_id").(string)
	threadID := 0
	var thread Thread
	json.Unmarshal(ctx.PostBody(), &thread)

	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
	if threadID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id FROM threads WHERE id=%d", id)).Scan(&threadID)
	}
	if threadID == 0 {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
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

	DB.Exec(context.Background(), fmt.Sprintf("UPDATE threads SET %s WHERE id=%d", setter, threadID))
	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", threadID)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(thread)
	return
}

func getForumUsers (ctx *fasthttp.RequestCtx) {
	forumSlug := ctx.UserValue("slug").(string)

	slug := ""
	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT slug FROM forums WHERE slug='%s'", forumSlug)).Scan(&slug)
	if slug == "" {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	limit := string(ctx.QueryArgs().Peek("limit"))
	since := string(ctx.QueryArgs().Peek("since"))
	desc, _ := strconv.ParseBool(string(ctx.QueryArgs().Peek("desc")))
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

	res, _ := DB.Query(context.Background(), query)
	if res == nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
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

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(users)
	return
}

func getPostInfo (ctx *fasthttp.RequestCtx) {
	postID, _ := strconv.Atoi(ctx.UserValue("id").(string))

	var post Post
	DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE id=%d", postID)).Scan(&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited, &post.Forum, &post.Thread, &post.Created)
	if post.ID == 0 {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"forum is not in system!",
		})
		return
	}

	jsonAnswer := make(map[string]interface{}, 0)
	jsonAnswer["post"] = post

	related := string(ctx.QueryArgs().Peek("related"))

	if strings.Contains(related, "user") {
		var user User
		DB.QueryRow(context.Background(), fmt.Sprintf("SELECT nickname, fullname, about, email FROM users WHERE nickname='%s'", post.Author)).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
		jsonAnswer["author"] = user
	}
	if strings.Contains(related, "thread") {
		var thread Thread
		DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", post.Thread)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
		jsonAnswer["thread"] = thread
	}
	if strings.Contains(related, "forum") {
		var forum Forum
		DB.QueryRow(context.Background(), fmt.Sprintf("SELECT title, \"user\", slug, posts, threads FROM forums WHERE slug='%s'", post.Forum)).Scan(&forum.Title, &forum.User, &forum.Slug, &forum.Posts, &forum.Threads)
		jsonAnswer["forum"] = forum
	}

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(jsonAnswer)
	return
}

func changePostMessage (ctx *fasthttp.RequestCtx) {
	postID, _ := strconv.Atoi(ctx.UserValue("id").(string))
	var newPost Post
	json.Unmarshal(ctx.PostBody(), &newPost)

	setQuery := ""
	if newPost.Message != "" {
		setQuery += fmt.Sprintf(" message = '%s'", newPost.Message)
	}
	if newPost.Author != "" {
		setQuery += fmt.Sprintf(" author = '%s',", newPost.Author)
	}
	if newPost.Parent != 0 {
		thread := 0
		DB.QueryRow(context.Background(), fmt.Sprintf("SELECT thread FROM posts WHERE id=%d", newPost.Parent)).Scan(&thread)
		if thread == newPost.Thread {
			setQuery += fmt.Sprintf(" parent = %d,", newPost.Parent)
		} else {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(ErrorMsg{
				"Parent post was created in another thread!",
			})
			return
		}
	}



	var err error
	if len(setQuery) > 0 {
		setQuery = strings.TrimRight(setQuery, ",") + " "
		err = DB.QueryRow(context.Background(), fmt.Sprintf("UPDATE posts SET %s WHERE id = %d RETURNING id, parent, author, message, isedited, forum, thread, created", setQuery, postID)).Scan(&newPost.ID, &newPost.Parent, &newPost.Author, &newPost.Message, &newPost.IsEdited, &newPost.Forum, &newPost.Thread, &newPost.Created)
	} else {
		err = DB.QueryRow(context.Background(), fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE id=%d", postID)).Scan(&newPost.ID, &newPost.Parent, &newPost.Author, &newPost.Message, &newPost.IsEdited, &newPost.Forum, &newPost.Thread, &newPost.Created)
	}
	if err != nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"post is not in system!",
		})
		return
	}

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(newPost)
	return
}

type Service struct {
	User int64 `json:"user"`
	Forum int64 `json:"forum"`
	Thread int64 `json:"thread"`
	Post int64 `json:"post"`

}
func getServiceStatus(ctx *fasthttp.RequestCtx) {
	var service Service

	DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM forums").Scan(&service.Forum)
	DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM threads").Scan(&service.Thread)
	DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM users").Scan(&service.User)
	DB.QueryRow(context.Background(), "SELECT COUNT(*) FROM posts").Scan(&service.Post)

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(service)
}