package main

import (
	"database/sql"
	"net/http"
)

func appRouter(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /tasks", getTasksHandler(db))
	mux.HandleFunc("POST /tasks", createTaskHandler(db))
	mux.HandleFunc("GET /tasks/{id}", taskItemHandler(db))
	mux.HandleFunc("PATCH /tasks/{id}", updateTaskHandler(db))
	mux.HandleFunc("DELETE /tasks/{id}", deleteTaskHandler(db))

	return mux
}
