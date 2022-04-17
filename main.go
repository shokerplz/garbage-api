package main

import (
        "log"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var sessionStorage map[string]string

type bookStruct struct {
	BookID	int
	GenreID	int
	AuthorID int
	AuthorName string
	BookName string
	Genre	string
	Year	int
	Score	int
}

type addBookStruct struct {
	User	string
	Token string
	BookSlice []addBookSliceStruct
}

type addBookSliceStruct struct {
        BookID int
        Score int
}

type userStruct struct {
	User     string
	Password string
}

type loggedUserStruct struct {
	User	string
	Token	string
}

func NewSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func PrettyPrint(v interface{}) (ans string) {
      b, err := json.MarshalIndent(v, "", "")
      if err == nil {
              return(string(b))
      }
      return ""
}

func logRequest(handler http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := ""
		for k, v := range r.Header {
			headers += k+":"+fmt.Sprint(v)+" "
		}
		sql := fmt.Sprintf("INSERT INTO logs(remote_addr, method, url, proto, headers, content_length, user_agent) VALUES ('%s', '%s', '%s', '%s', '%s', '%d', '%s')", r.RemoteAddr, r.Method, r.URL, r.Proto, headers, r.ContentLength, r.Header["User-Agent"])
		_, err := db.Exec(sql)
		if err != nil {
			log.Print("Can't write logs to DB")
		}
		log.Printf(`{"remote_addr" : "%s", "method": "%s", "url": "%s", "proto" : "%s", "headers" : "%s", "content_length" : "%d", "user_agent" : "%s"}`, r.RemoteAddr, r.Method, r.URL, r.Proto, headers, r.ContentLength, r.Header["User-Agent"])
		handler.ServeHTTP(w, r)
	})
}

func register(w http.ResponseWriter, req *http.Request, db *sql.DB) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var user userStruct
	err = json.Unmarshal(body, &user)
	if err != nil {
		panic(err)
	}
	pwdHash := NewSHA256([]byte(user.Password))
	sql := fmt.Sprintf("INSERT INTO user(user, password) VALUES ('%s', '%s')", user.User, hex.EncodeToString(pwdHash))
	_, err = db.Exec(sql)
	if err != nil {
		http.Error(w, `{ "status":"error", "msg": "User with such name already exists" }`, http.StatusBadRequest)
		return
	} else {
		fmt.Fprintf(w, `{ "status":"ok", "msg": "You are successfully registered" }`)
	}
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func GenerateAuthCookie(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func login(w http.ResponseWriter, req *http.Request, db *sql.DB) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var user userStruct
	err = json.Unmarshal(body, &user)
	if err != nil {
		panic(err)
	}
	pwdHash := NewSHA256([]byte(user.Password))
	sql := fmt.Sprintf("SELECT user FROM user WHERE user = '%s' and password = '%s'", user.User, hex.EncodeToString(pwdHash))
	err = db.QueryRow(sql).Scan(&user.User)
	if err != nil {
		fmt.Println(err)
		http.Error(w, `{ "status":"error", "msg": "Username or password is incorrect" }`, http.StatusUnauthorized)
		return
	} else {
		sessionStorage[user.User] = GenerateAuthCookie(16)
		fmt.Fprintf(w, `{ "status":"ok", "msg": "You have successfully logged in", "cookie": "`+sessionStorage[user.User]+`" }`)
	}
}

func checkAuthToken(user string, token string) bool {
        return sessionStorage[user] == token
}

func getBooks(w http.ResponseWriter, req *http.Request, db *sql.DB) {
	body, err := ioutil.ReadAll(req.Body)
        if err != nil {
                panic(err)
        }
        var user loggedUserStruct
        err = json.Unmarshal(body, &user)
        if err != nil {
                panic(err)
        }
	if !checkAuthToken(user.User, user.Token) {
		http.Error(w, `{ "status":"error", "msg": "User token is invalid" }`, http.StatusUnauthorized)
		return
		return
	}
	sql := fmt.Sprintf("select book.book_id, genres.genre_id, authors.author_id, authors.name as authorName, book.name as bookName,  genres.name as genre, year, score from userbooklist inner join book on userbooklist.book_id = book.book_id inner join authors on book.author_id = authors.author_id inner join genres on book.genre_id = genres.genre_id where user = '%s'", user.User)
	rows, err := db.Query(sql)
	if err != nil {
		fmt.Println(err)
		http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
	}
	bookSlice := []bookStruct{}
	for rows.Next() {
		var bookid int
		var genreid int
		var authorid int
		var bookname string
		var authorname string
		var genre string
		var year int
		var score int
		err_scan := rows.Scan(&bookid, &genreid, &authorid, &authorname, &bookname, &genre, &year, &score)
		if err_scan != nil {
			fmt.Println(err)
			http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
			return
		}
		book := bookStruct{bookid, genreid, authorid, bookname, authorname, genre, year, score}
		bookSlice = append(bookSlice, book)
	}
	j, err_j := json.Marshal(bookSlice)
	if err_j != nil {
		fmt.Println(err)
		http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, `{ "status":"ok", "book_list": `+string(j)+`}`)
}

func getAllBooks(w http.ResponseWriter, req *http.Request, db *sql.DB) {
	body, err := ioutil.ReadAll(req.Body)
        if err != nil {
                panic(err)
        }
        var user loggedUserStruct
        err = json.Unmarshal(body, &user)
        if err != nil {
                panic(err)
        }
        if !checkAuthToken(user.User, user.Token) {
                http.Error(w, `{ "status":"error", "msg": "User token is invalid" }`, http.StatusUnauthorized)
		return
                return
        }
	sql := "select book.book_id, genres.genre_id, authors.author_id, authors.name as authorName, book.name as bookName, genres.name as genre, book.year from book inner join authors on book.author_id = authors.author_id inner join genres on genres.genre_id = book.genre_id"
        rows, err := db.Query(sql)
        if err != nil {
                fmt.Println(err)
                http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
        }
        bookSlice := []bookStruct{}
        for rows.Next() {
                var bookid int
                var genreid int
                var authorid int
                var bookname string
                var authorname string
                var genre string
                var year int
                err_scan := rows.Scan(&bookid, &genreid, &authorid, &authorname, &bookname, &genre, &year)
                if err_scan != nil {
                        fmt.Println(err)
                        http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
			return
                }
                book := bookStruct{bookid, genreid, authorid, bookname, authorname, genre, year, 0}
                bookSlice = append(bookSlice, book)
        }
        j, err_j := json.Marshal(bookSlice)
        if err_j != nil {
                fmt.Println(err)
                http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
        }
        fmt.Fprintf(w, `{ "status":"ok", "book_list": `+string(j)+`}`)
}

func addBook(w http.ResponseWriter, req *http.Request, db *sql.DB) {
        body, err := ioutil.ReadAll(req.Body)
        if err != nil {
                fmt.Println(err)
		http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
        }
        var bodyStruct addBookStruct
        err = json.Unmarshal(body, &bodyStruct)
        if err != nil {
                fmt.Println(err)
                http.Error(w, `{ "status":"error", "msg": "Wrong JSON sent" }`, http.StatusBadRequest)
		return
        }
        if !checkAuthToken(bodyStruct.User, bodyStruct.Token) {
                http.Error(w, `{ "status":"error", "msg": "User token is invalid" }`, http.StatusUnauthorized)
		return
        }
	sql := "INSERT INTO userbooklist(user, book_id, score) VALUES "
	for idx, book := range bodyStruct.BookSlice {
		sql = sql + fmt.Sprintf("('%s', %d, %d)", bodyStruct.User, book.BookID ,book.Score)
		if idx != len(bodyStruct.BookSlice) - 1 {
			sql = sql + ", "
		}
	}
        _, err = db.Exec(sql)
        if err != nil {
		fmt.Println(err)
        	http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
        }
	fmt.Fprintf(w, `{ "status":"ok", "msg" : "You have successfully added books" }`)
}

func deleteBooks(w http.ResponseWriter, req *http.Request, db *sql.DB) {
        body, err := ioutil.ReadAll(req.Body)
        if err != nil {
                panic(err)
        }
        var user loggedUserStruct
        err = json.Unmarshal(body, &user)
        if err != nil {
                panic(err)
        }
        if !checkAuthToken(user.User, user.Token) {
                http.Error(w, `{ "status":"error", "msg": "User token is invalid" }`, http.StatusUnauthorized)
                return
        }
	sql := fmt.Sprintf("DELETE FROM userbooklist WHERE user = '%s'", user.User)
	_, err = db.Exec(sql)
        if err != nil {
                fmt.Println(err)
                http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
        }
        fmt.Fprintf(w, `{ "status":"ok", "msg" : "You have successfully deletedd books" }`)
}

func getRecomendations(w http.ResponseWriter, req *http.Request, db *sql.DB) {
        body, err := ioutil.ReadAll(req.Body)
        if err != nil {
                panic(err)
        }
        var user loggedUserStruct
        err = json.Unmarshal(body, &user)
        if err != nil {
                panic(err)
        }
        if !checkAuthToken(user.User, user.Token) {
                http.Error(w, `{ "status":"error", "msg": "User token is invalid" }`, http.StatusUnauthorized)
                return
        }
	book1 := bookStruct{0, 0, 0, "Grisham, John", "The Rooster Bar", "Fantasy", 2018, 0}
	book2 := bookStruct{0, 0, 0, "Moore, Christopher", "Lamb", "Fiction", 1956, 0}
	book3 := bookStruct{0, 0, 0, "Barbery, Muriel", "The Elegance of the Hedgehog", "Dystopian", 2008, 0}
	book4 := bookStruct{0, 0, 0, "Preston, Richard", "Crisis in the Red Zone", "Dystopian", 2000, 0}
	book5 := bookStruct{0, 0, 0, "Flood, John", "Bag Men", "Western", 1997, 0}
	book6 := bookStruct{0, 0, 0, "Bach, Sebastian", "18 and Life on Skid Row", "Horror", 2016, 0}
	book7 := bookStruct{0, 0, 0, "Dershowitz, Alan M.", "The Advocate's Devil", "Literature", 1994, 0}
	randomList := []bookStruct{
		book1,
		book2,
		book3,
		book4,
		book5,
		book6,
		book7,
	}
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)
	recomendation := randomList[r.Intn(len(randomList))]
        j, err_j := json.Marshal(recomendation)
        if err_j != nil {
                fmt.Println(err)
                http.Error(w, `{ "status":"error", "msg": "Server side error" }`, http.StatusInternalServerError)
		return
        }
	fmt.Fprintf(w, `{ "status":"ok", "recomendation" : [`+string(j)+`] }`)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	db, err := sql.Open("mysql", "bookking:bookking@tcp(mysql:3306)/bookking")
	if err != nil {
		panic(err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	sessionStorage = map[string]string{}
	http.HandleFunc("/api/v1/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
		}
		register(w, r, db)
	})
	http.HandleFunc("/api/v1/login", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodPost {
                        http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
                }
		login(w, r, db)
	})
        http.HandleFunc("/api/v1/my/get-books", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodPost {
                        http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
                }
		getBooks(w, r, db)
	})
	http.HandleFunc("/api/v1/pubic/get-books", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodPost {
                        http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
                }
		getAllBooks(w, r, db)
	})
	http.HandleFunc("/api/v1/my/add-book", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodPut {
                        http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
                }
		addBook(w, r, db)
	})
	http.HandleFunc("/api/v1/my/delete-books", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodDelete {
                        http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
                }
		deleteBooks(w, r, db)
	})
	http.HandleFunc("/api/v1/my/get-recomendations", func(w http.ResponseWriter, r *http.Request) {
                if r.Method != http.MethodPost {
                        http.Error(w, `{ "status":"error", "msg": "Method not allowed" }`, http.StatusMethodNotAllowed)
			return
                }
		getRecomendations(w, r, db)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Bookking")
	})
	http.ListenAndServe(":8080", logRequest(http.DefaultServeMux, db))
}
