package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

var DEFAULT_TIMEOUT = 2 * time.Second

type Task struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type UpdateTaskRequest struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

func trimAndValidate(rawStr string) (string, error) {
	str := strings.TrimSpace(rawStr)
	length := utf8.RuneCountInString(str)
	if length == 0 {
		return "", errors.New("Length must be greater than 0")
	}
	if length > 50 {
		return "", errors.New("Length must be less than 50")
	}
	return str, nil
}

func getTasksHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tasks = make([]Task, 0)
		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()

		rows, err := db.QueryContext(ctx, "SELECT id, title, completed FROM tasks")

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer rows.Close()

		for rows.Next() {
			var task = Task{}
			if err := rows.Scan(&task.ID, &task.Title, &task.Completed); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tasks = append(tasks, task)
		}

		if err = rows.Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}
}

func createTaskHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var task Task

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&task); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			http.Error(w, "Request must contain exactly one JSON object", http.StatusBadRequest)
			return
		}

		title, err := trimAndValidate(task.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		task.Title = title

		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()
		result, err := db.ExecContext(ctx, "INSERT INTO tasks (title, completed) VALUES (?, ?)", task.Title, task.Completed)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		task.ID = int(newID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(task)
	}
}

func updateTaskHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var updatedTask UpdateTaskRequest
		var task Task

		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		if err := decoder.Decode(&updatedTask); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var temp any
		if err := decoder.Decode(&temp); !errors.Is(err, io.EOF) {
			http.Error(w, "Request must contain exactly one JSON object", http.StatusBadRequest)
			return
		}

		if updatedTask.Title == nil && updatedTask.Completed == nil {
			http.Error(w, "JSON can't be empty", http.StatusBadRequest)
			return
		}

		if updatedTask.Title != nil {
			title, err := trimAndValidate(*updatedTask.Title)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			updatedTask.Title = &title
		}

		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()

		_, err = db.ExecContext(ctx, "UPDATE tasks SET title = COALESCE (?, title), completed = COALESCE(?, completed) WHERE id = ?", updatedTask.Title, updatedTask.Completed, id)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				http.Error(w, "Request canceled", http.StatusServiceUnavailable)
			} else if errors.Is(err, context.DeadlineExceeded) {
				http.Error(w, "Request timed out", http.StatusServiceUnavailable)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		err = db.QueryRowContext(ctx, "SELECT id, title, completed FROM tasks WHERE id = ?", id).Scan(&task.ID, &task.Title, &task.Completed)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "Tasks doesn't exist", http.StatusNotFound)
				return
			}
			http.Error(w, "Failed to fetch edited task", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	}
}

func deleteTaskHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()
		result, err := db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		changedCount, err := result.RowsAffected()

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if changedCount == 0 {
			http.Error(w, "task not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func taskItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var task = Task{}

		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()
		err = db.QueryRowContext(ctx, "SELECT id, title, completed FROM tasks WHERE id = ?;", id).Scan(&task.ID, &task.Title, &task.Completed)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	}
}

func connectDB() *sql.DB {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal("Error loading .env: ", err)
	}
	username := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	config := fmt.Sprintf("%s:%s@/%s", username, password, dbName)
	db, err := sql.Open("mysql", config)
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err.Error())
	}
	return db
}

func main() {
	db := connectDB()
	defer db.Close()
	mux := http.NewServeMux()
	port := ":8000"
	mux.HandleFunc("GET /tasks", getTasksHandler(db))
	mux.HandleFunc("POST /tasks", createTaskHandler(db))
	mux.HandleFunc("GET /tasks/{id}", taskItemHandler(db))
	mux.HandleFunc("PATCH /tasks/{id}", updateTaskHandler(db))
	mux.HandleFunc("DELETE /tasks/{id}", deleteTaskHandler(db))
	fmt.Printf("Server running on port %s\n", port[1:])

	log.Fatal(http.ListenAndServe(port, mux))
}
