package main

import (
	"encoding/json"
	"fmt"
	"github.com/fasthttp/router"
	"github.com/go-openapi/strfmt"
	_ "github.com/jackc/pgconn"
	"github.com/jackc/pgx"
	"github.com/valyala/fasthttp"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
	defer DB.Close()
	//
	//path := filepath.Join("script.sql")
	//c, _ := ioutil.ReadFile(path)
	//scriptString := string(c)
	//DB.Exec(scriptString)


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
}

var (
	strContentType = []byte("Content-Type")
	strApplicationJSON = []byte("application/json")
)

func dbClearAll(ctx *fasthttp.RequestCtx)  {
	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	_, err := transactionConnection.Exec(`TRUNCATE users CASCADE;`)
	_, err = transactionConnection.Exec(`TRUNCATE forums CASCADE;`)
	_, err = transactionConnection.Exec(`TRUNCATE threads CASCADE;`)
	_, err = transactionConnection.Exec(`TRUNCATE votes CASCADE;`)
	_, err = transactionConnection.Exec(`TRUNCATE posts CASCADE;`)
	if err != nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusInternalServerError)
	}
	transactionConnection.Commit()
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

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	_, err := transactionConnection.Exec(`INSERT INTO users(nickname, fullname, about, email) VALUES ($1, $2, $3, $4)`, user.Nickname, user.Fullname, user.About, user.Email)

	if err != nil  {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			res, _ := DB.Query(`SELECT nickname, fullname, about, email FROM users WHERE email=$1 or nickname=$2`, user.Email, user.Nickname)

			users := make([]User, 0)
			for res.Next() {
				var user User
				res.Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
				users = append(users, user)
			}
			res.Close()
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(users)
			return
		}
	}

	transactionConnection.Commit()
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

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	_, err := transactionConnection.Exec(`INSERT INTO forums(title, "user", slug) VALUES ($1, $2, $3)`, forum.Title, forum.User, forum.Slug)

	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			DB.QueryRow(`SELECT title, "user", slug, posts, threads FROM forums WHERE slug=$1`, forum.Slug).Scan(&forum.Title, &forum.User,  &forum.Slug, &forum.Posts, &forum.Threads)
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(forum)
			return
		}
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23503" {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusNotFound)
			json.NewEncoder(ctx).Encode(forum)
			return
		}
	}


	transactionConnection.QueryRow(`SELECT nickname FROM users WHERE nickname=$1`, forum.User).Scan(&forum.User)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(forum)
	transactionConnection.Commit()
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
	Created strfmt.DateTime `json:"created"`
}

func createThread(ctx *fasthttp.RequestCtx) {
	var thread Thread
	thread.Forum = ctx.UserValue("slug").(string)
	json.Unmarshal(ctx.PostBody(), &thread)


	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	if thread.Slug != "" {
		transactionConnection.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE slug=$1`, thread.Slug).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
		if thread.ID != 0 {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(thread)
			return
		}
	}

	_, err := transactionConnection.Exec(`INSERT INTO threads(title, author, forum, message, votes, slug, created) VALUES ($1, $2, $3, $4, $5, $6, $7)`, thread.Title, thread.Author, thread.Forum, thread.Message, thread.Votes, thread.Slug, thread.Created)
	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23503" {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusNotFound)
			json.NewEncoder(ctx).Encode(ErrorMsg{
				"Author or forum slug doesnt exists",
			})
			return
		}
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(thread)
			return
		}
	}

	transactionConnection.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE forum=$1 AND author=$2 AND title=$3`, thread.Forum, thread.Author, thread.Title).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	transactionConnection.QueryRow(`SELECT slug FROM forums WHERE slug=$1`, thread.Forum).Scan(&thread.Forum)

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(thread)
	transactionConnection.Commit()
	return
}

type Vote struct {
	Nickname string `json:"nickname"`
	Voice int `json:"voice"`
	ThreadID int `json:"thread_id"`
}

func voteThread (ctx *fasthttp.RequestCtx) {
	var vote Vote
	slugOrId := ctx.UserValue("slug_or_id").(string)
	json.Unmarshal(ctx.PostBody(), &vote)
	slugOrIdConverted, _ := strconv.Atoi(slugOrId)

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	transactionConnection.QueryRow(`SELECT id FROM threads WHERE slug=$1 or id=$2`, slugOrId, slugOrIdConverted).Scan(&vote.ThreadID)

	_, err := transactionConnection.Exec(`INSERT INTO votes(nickname, voice, threadID) VALUES ($1, $2, $3)`, vote.Nickname, vote.Voice, vote.ThreadID)

	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23503" {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusNotFound)
			json.NewEncoder(ctx).Encode(ErrorMsg{
				"cant find thread!",
			})
			return
		}
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			DB.Exec(`UPDATE votes SET voice = $1 WHERE "threadid" = $2 AND nickname = $3;`, vote.Voice, vote.ThreadID, vote.Nickname)
		}
	}

	transactionConnection.Commit()
	var thread Thread
	DB.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=$1`, vote.ThreadID).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
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
	Created strfmt.DateTime `json:"created"`
}

func createPost (ctx *fasthttp.RequestCtx) {
	threadSlugOrId := ctx.UserValue("slug_or_id").(string)
	threadSlugOrIdConverted, convertErr := strconv.Atoi(threadSlugOrId)
	threadID, forumSlug := -1, ""

	var err error
	if convertErr != nil {
		err = DB.QueryRow(`SELECT id, forum FROM threads WHERE slug=$1`, threadSlugOrId).Scan(&threadID, &forumSlug)
	} else {
		err = DB.QueryRow(`SELECT id, forum FROM threads WHERE id=$1`, threadSlugOrIdConverted).Scan(&threadID, &forumSlug)
	}
	if err != nil {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"cant find thread!",
		})
		return
	}


	posts := make([]Post, 0)
	json.Unmarshal(ctx.PostBody(), &posts)
	if len(posts) == 0 {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusCreated)
		json.NewEncoder(ctx).Encode(posts)
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
		err = DB.QueryRow(`SELECT thread FROM posts WHERE id = $1`,posts[0].Parent).Scan(&pThread)

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


	timeOfCreation := strfmt.DateTime(time.Now())
	resultQueryString := `INSERT INTO posts(parent, author, message, thread, forum, created) VALUES `
	queryArguments := make([]interface{}, len(posts)*6)
	//var queryArguments []interface{}
	// TODO: Validate PARENTS POST SOMEHOW!
	for index, _ := range posts {
		posts[index].Forum = forumSlug
		posts[index].Thread = threadID
		posts[index].Created = timeOfCreation

		resultQueryString += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d),", index*6+1, index*6+2, index*6+3, index*6+4, index*6+5, index*6+6)
		queryArguments [index*6] = posts[index].Parent
		queryArguments [index*6 + 1] = posts[index].Author
		queryArguments [index*6 + 2] = posts[index].Message
		queryArguments [index*6 + 3] = posts[index].Thread
		queryArguments [index*6 + 4] = posts[index].Forum
		queryArguments [index*6 + 5] = posts[index].Created

	}
	resultQueryString = strings.TrimRight(resultQueryString, ",") + " RETURNING id;"

	//start := time.Now()
	res, _ := DB.Query(resultQueryString, queryArguments...)
	//fmt.Printf("Query time: %s\n", time.Since(start))

	for index, _ := range posts {
		res.Next()
		err = res.Scan(&posts[index].ID)
	}
	if err != nil {
		res.Close()
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"cant find something!",
		})
		return
	}

	res.Close()
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusCreated)
	json.NewEncoder(ctx).Encode(posts)
	return
}

func getUserInfo (ctx *fasthttp.RequestCtx) {
	var user User
	nickname := ctx.UserValue("nickname").(string)


	DB.QueryRow(`SELECT nickname, fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
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

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	if user == (User{}) {
		transactionConnection.QueryRow(`SELECT fullname, about, email FROM users WHERE nickname=$1`, nickname).Scan(&user.Fullname, &user.About, &user.Email)
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
	err := transactionConnection.QueryRow(fmt.Sprintf("UPDATE users SET %s  WHERE nickname='%s' RETURNING fullname, about, email", setQuery, user.Nickname)).Scan(&updatedUser.Fullname, &updatedUser.About, &updatedUser.Email)
	if err != nil {
		if errPg, _ := err.(pgx.PgError); errPg.Code == "23505" {
			ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
			ctx.Response.SetStatusCode(http.StatusConflict)
			json.NewEncoder(ctx).Encode(ErrorMsg{"Conflict with other user"})
			return
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
	transactionConnection.Commit()
	return
}

func getForumInfo (ctx *fasthttp.RequestCtx) {
	var forum Forum
	slug := ctx.UserValue("slug").(string)

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	transactionConnection.QueryRow(fmt.Sprintf("SELECT title, \"user\", slug, posts, threads FROM forums WHERE slug='%s'", slug)).Scan(&forum.Title, &forum.User, &forum.Slug, &forum.Posts, &forum.Threads)
	if (Forum{}) == forum {
		ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
		ctx.Response.SetStatusCode(http.StatusNotFound)
		json.NewEncoder(ctx).Encode(ErrorMsg{
			"forum not found!",
		})
		return
	}

	transactionConnection.QueryRow(`SELECT nickname FROM users WHERE nickname=$1`, forum.User).Scan(&forum.User)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(forum)
	return

}

func getThreadsInfo (ctx *fasthttp.RequestCtx) {
	forumSlug := ctx.UserValue("slug").(string)

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	err := transactionConnection.QueryRow(`SELECT slug FROM forums WHERE slug=$1`, forumSlug).Scan(&forumSlug)
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

	res, _ := transactionConnection.Query(query)
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
	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	transactionConnection.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE slug=$1`, threadSlugOrId).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	if thread.ID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		transactionConnection.QueryRow(`SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=$1`, id).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
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

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	transactionConnection.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
	if threadID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		transactionConnection.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE id=%d", id)).Scan(&threadID)
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

	res, _ := transactionConnection.Query(query)
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

	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	transactionConnection.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE slug='%s'", threadSlugOrId)).Scan(&threadID)
	if threadID == 0 {
		id, _ := strconv.Atoi(threadSlugOrId)
		transactionConnection.QueryRow(fmt.Sprintf("SELECT id FROM threads WHERE id=%d", id)).Scan(&threadID)
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

	transactionConnection.Exec(fmt.Sprintf("UPDATE threads SET %s WHERE id=%d", setter, threadID))
	transactionConnection.Commit()
	DB.QueryRow(fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", threadID)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(thread)
	return
}

func getForumUsers (ctx *fasthttp.RequestCtx) {
	forumSlug := ctx.UserValue("slug").(string)
	slug := ""
	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	transactionConnection.QueryRow(fmt.Sprintf("SELECT slug FROM forums WHERE slug='%s'", forumSlug)).Scan(&slug)
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

	res, _ := transactionConnection.Query(query)
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
	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()
	transactionConnection.QueryRow(fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE id=%d", postID)).Scan(&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited, &post.Forum, &post.Thread, &post.Created)
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
		transactionConnection.QueryRow(fmt.Sprintf("SELECT nickname, fullname, about, email FROM users WHERE nickname='%s'", post.Author)).Scan(&user.Nickname, &user.Fullname, &user.About, &user.Email)
		jsonAnswer["author"] = user
	}
	if strings.Contains(related, "thread") {
		var thread Thread
		transactionConnection.QueryRow(fmt.Sprintf("SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id=%d", post.Thread)).Scan(&thread.ID, &thread.Title, &thread.Author, &thread.Forum, &thread.Message, &thread.Votes, &thread.Slug, &thread.Created)
		jsonAnswer["thread"] = thread
	}
	if strings.Contains(related, "forum") {
		var forum Forum
		transactionConnection.QueryRow(fmt.Sprintf("SELECT title, \"user\", slug, posts, threads FROM forums WHERE slug='%s'", post.Forum)).Scan(&forum.Title, &forum.User, &forum.Slug, &forum.Posts, &forum.Threads)
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
		DB.QueryRow(fmt.Sprintf("SELECT thread FROM posts WHERE id=%d", newPost.Parent)).Scan(&thread)
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
		err = DB.QueryRow(fmt.Sprintf("UPDATE posts SET %s WHERE id = %d RETURNING id, parent, author, message, isedited, forum, thread, created", setQuery, postID)).Scan(&newPost.ID, &newPost.Parent, &newPost.Author, &newPost.Message, &newPost.IsEdited, &newPost.Forum, &newPost.Thread, &newPost.Created)
	} else {
		err = DB.QueryRow(fmt.Sprintf("SELECT id, parent, author, message, isedited, forum, thread, created FROM posts WHERE id=%d", postID)).Scan(&newPost.ID, &newPost.Parent, &newPost.Author, &newPost.Message, &newPost.IsEdited, &newPost.Forum, &newPost.Thread, &newPost.Created)
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
	transactionConnection, _ := DB.Begin()
	defer transactionConnection.Rollback()

	transactionConnection.QueryRow("SELECT COUNT(*) FROM forums").Scan(&service.Forum)
	transactionConnection.QueryRow("SELECT COUNT(*) FROM threads").Scan(&service.Thread)
	transactionConnection.QueryRow("SELECT COUNT(*) FROM users").Scan(&service.User)
	transactionConnection.QueryRow("SELECT COUNT(*) FROM posts").Scan(&service.Post)

	ctx.Response.Header.SetCanonical(strContentType, strApplicationJSON)
	ctx.Response.SetStatusCode(http.StatusOK)
	json.NewEncoder(ctx).Encode(service)
}