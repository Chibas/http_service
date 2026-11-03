package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

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
		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()

		tasks, err := getTasks(ctx, db)

		if err != nil {
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, "request timeout", http.StatusGatewayTimeout)
				return
			case errors.Is(err, context.Canceled):
				return // canceled
			default:
				log.Printf("getTasks failed: %v", err)
				http.Error(w, "Task fetching failed", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tasks)
	}
}

func createTaskHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var taskUpdate UpdateTaskRequest

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&taskUpdate); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			http.Error(w, "Request must contain exactly one JSON object", http.StatusBadRequest)
			return
		}

		if taskUpdate.Title == nil {
			http.Error(w, "Empty title", http.StatusBadRequest)
			return
		}
		title, err := trimAndValidate(*taskUpdate.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()

		task, err := createTask(ctx, db, title)
		if err != nil {
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, "request timeout", http.StatusGatewayTimeout)
				return
			case errors.Is(err, context.Canceled):
				return // nothing to do for canceled client
			default:
				http.Error(w, "Task creation failed", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(task)
	}
}

func updateTaskHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var updatedTask UpdateTaskRequest

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

		task, err := updateTask(ctx, db, id, updatedTask)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				http.Error(w, "task not found", http.StatusNotFound)
			case errors.Is(err, context.Canceled):
				return
			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, "request timeout", http.StatusGatewayTimeout)
			default:
				log.Printf("updateTask failed: %v", err)
				http.Error(w, "Task update failed", http.StatusInternalServerError)
			}
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

		isDeleted, err := deleteTask(ctx, db, id)
		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				return
			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, "request timeout", http.StatusGatewayTimeout)
				return
			default:
				log.Printf("deleteTask failed: %v", err)
				http.Error(w, "failed to delete task", http.StatusInternalServerError)
				return
			}
		}
		if isDeleted {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "failed to delete task", http.StatusNotFound)
	}
}

func taskItemHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), DEFAULT_TIMEOUT)
		defer cancel()

		task, err := getTaskByID(ctx, db, id)
		if err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				http.Error(w, "not found", http.StatusNotFound)

			case errors.Is(err, context.DeadlineExceeded):
				http.Error(w, "request timeout", http.StatusGatewayTimeout)

			case errors.Is(err, context.Canceled):
				return // No response for disconnected client

			default:
				log.Printf("getTaskByID failed: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(task)
	}
}
