package main

import (
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

type userStruct struct {
	User     string
	Password string
}

func NewSHA256(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
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
		http.Error(w, "User with such name already exists", http.StatusBadRequest)
	} else {
		fmt.Fprintf(w, "You are successfully registered")
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
		http.Error(w, "Username or password is incorrect", http.StatusBadRequest)
	} else {
		sessionStorage[user.User] = GenerateAuthCookie(16)
		fmt.Fprintf(w, "You have successfully logged in. Your cookie: "+sessionStorage[user.User])
	}
}

func main() {
	db, err := sql.Open("mysql", "bookking:bookking@tcp(127.1:3306)/bookking")
	if err != nil {
		panic(err)
	}
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	sessionStorage = map[string]string{}
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		register(w, r, db)
	})
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		login(w, r, db)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World!")
	})
	http.ListenAndServe(":8080", nil)
}
