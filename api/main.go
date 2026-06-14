package handler

import (
	"database/sql"
	"net/http"
	"sync"
)

var (
	db     *sql.DB
	once   sync.Once
	router http.Handler
)
