package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func testConnectDB(t *testing.T) *sql.DB {
	t.Helper()
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("Error loading .env: %v", err)
	}
	username := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_TEST_NAME")
	if dbName == "" {
		t.Fatalf("Couldn't get DB name")
	}
	config := fmt.Sprintf("%s:%s@/%s", username, password, dbName)
	db, err := sql.Open("mysql", config)
	if err != nil {
		t.Fatalf("Failed to open the test database: %v", err)
	}

	err = db.Ping()
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}
	})
	return db
}

func TestGetTaskInvalidId(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"Non-numeric ID", "/tasks/abc", http.StatusBadRequest},
		{"Mixed ID", "/tasks/123abc", http.StatusBadRequest},
		{"Decimal ID", "/tasks/1.5", http.StatusBadRequest},
		{"Overflow ID", "/tasks/999999999999999999999999", http.StatusBadRequest},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks/{id}", taskItemHandler(nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			req := httptest.NewRequest(
				http.MethodGet,
				tt.path,
				nil,
			)

			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Errorf(
					"Expected status %d, got %d",
					tt.wantStatus,
					resp.Code,
				)
			}
		})
	}
}

func TestCreateTaskInvalidJson(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"Malformed JSON", "{\"title\":", http.StatusBadRequest},
		{"Not JSON", "invalid json", http.StatusBadRequest},
		{"Not a string", "{\"title\": 123}", http.StatusBadRequest},
		{"Empty body", "", http.StatusBadRequest},
	}

	path := "/tasks"
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", createTaskHandler(nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				path,
				strings.NewReader(tt.body),
			)

			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Errorf("Expected status %d got %d", tt.wantStatus, resp.Code)
			}
		})
	}
}

func TestCreateTaskSuccess(t *testing.T) {

	test := struct {
		name       string
		path       string
		body       string
		wantStatus int
	}{
		"Create task",
		"/tasks",
		"{\"title\": \"test_name\", \"completed\": false}",
		http.StatusCreated,
	}
	var result Task
	db := testConnectDB(t)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", createTaskHandler(db))

	t.Run(test.name, func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)

		if resp.Code != test.wantStatus {
			t.Fatalf("Expected status %d got %d", test.wantStatus, resp.Code)
		}

		err := json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			t.Fatalf("Decoding JSON failed with %v", err)
		}

		if result.ID <= 0 {
			t.Fatalf("result ID should be greater than 0")
		}

		if result.Title != "test_name" {
			t.Errorf("Incorrect title, expected %s, got %s", "test_name", result.Title)
		}

		if result.Completed != false {
			t.Errorf("Incorrect status, expected %v, got %v", false, result.Completed)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		t.Cleanup(func() {
			_, err := db.Exec(
				"DELETE FROM tasks WHERE id = ?",
				result.ID,
			)
			if err != nil {
				t.Errorf("Failed to clean up task: %v", err)
			}
		})

		var rowsCount int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) from tasks WHERE id = ?", result.ID).Scan(&rowsCount)

		if err != nil {
			t.Fatalf("Quering DB failed with an error %v", err)
		}

		if rowsCount != 1 {
			t.Fatalf("Invalid error count, expected 1, got %d", rowsCount)
		}
	})
}

func TestGetTaskSuccess(t *testing.T) {
	test := struct {
		name       string
		title      string
		wantStatus int
	}{
		"Get single task by ID",
		"test_get_task",
		200,
	}

	db := testConnectDB(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks/{id}", taskItemHandler(db))

	t.Run(test.name, func(t *testing.T) {
		ctx := context.Background()

		result, err := db.ExecContext(ctx, "INSERT INTO tasks ( title, completed) VALUES (?, ?)", test.title, true)
		if err != nil {
			t.Fatalf("Failed to insert test data into DB: %v", err)
		}
		taskID, err := result.LastInsertId()
		if err != nil {
			t.Errorf("Failed to fetch LastInsertedID")
		}
		t.Cleanup(func() {
			_, err := db.Exec(
				"DELETE FROM tasks WHERE id = ?",
				taskID,
			)
			if err != nil {
				t.Errorf("Failed to clean up task: %v", err)
			}
		})

		path := fmt.Sprintf("/tasks/%d", taskID)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)

		var task Task
		err = json.NewDecoder(resp.Body).Decode(&task)
		if err != nil {
			t.Fatalf("JSON response decoding failed")
		}

		if taskID != int64(task.ID) {
			t.Fatalf("Task ID did not match DB")
		}

		if resp.Code != test.wantStatus {
			t.Errorf("Request was not successful")
		}

		if task.Title != test.title {
			t.Errorf("Title did not match")
		}
	})

}

func TestDeleteTaskSuccess(t *testing.T) {
	test := struct {
		name       string
		title      string
		wantStatus int
	}{
		"Delete task from DB",
		"test_task_name",
		http.StatusNoContent,
	}

	db := testConnectDB(t)
	mux := http.NewServeMux()

	mux.HandleFunc("DELETE /tasks/{id}", deleteTaskHandler(db))

	t.Run(test.name, func(t *testing.T) {

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		result, err := db.ExecContext(ctx, "INSERT INTO tasks (title, completed) VALUES (?, ?)", test.title, false)
		if err != nil {
			t.Fatalf("DB inseertion failed: %v", err)
		}

		taskID, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("Failed to retreive lastInsertedId: %v", err)
		}

		t.Cleanup(func() {
			_, err := db.Exec(
				"DELETE FROM tasks WHERE id = ?",
				taskID,
			)
			if err != nil {
				t.Errorf("Failed to clean up task: %v", err)
			}
		})

		path := fmt.Sprintf("/tasks/%d", taskID)
		req := httptest.NewRequest(http.MethodDelete, path, nil)
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)

		if res.Code != test.wantStatus {
			t.Fatalf("Response code fail. Expected %d, got %d", test.wantStatus, res.Code)
		}

		if res.Body.Len() != 0 {
			t.Errorf(
				"Expected empty response body, got %q",
				res.Body.String(),
			)
		}

		var id int
		err = db.QueryRowContext(ctx, "SELECT id FROM tasks WHERE id = ?", taskID).Scan(&id)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return
		case err != nil:
			t.Fatalf("Database verification failed: %v", err)
		default:
			t.Fatalf("Task %d still exists in the DB after DELETE", taskID)
		}
	})
}

func TestUpdateTaskSuccess(t *testing.T) {
	tests := []struct {
		name          string
		initialTitle  string
		initialStatus bool
		body          string
		wantTitle     string
		wantCompleted bool
	}{
		{"Update title", "original", false, "{\"title\": \"updated\"}", "updated", false},
		{
			"Update status",
			"original",
			true,
			"{\"completed\":false}",
			"original",
			false,
		},
		{"Updated both", "original", false, "{\"title\": \"updated\", \"completed\": true}", "updated", true},
	}
	// CONNECT TO DB AND CREATE ENDPOINT
	db := testConnectDB(t)
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /tasks/{id}", updateTaskHandler(db))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// INSERT TEST DATA INTO DB
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			result, err := db.ExecContext(ctx, "INSERT INTO tasks (title, completed) VALUES (?, ?)", tt.initialTitle, tt.initialStatus)
			if err != nil {
				t.Fatalf("Failed to insert initial task: %v", err)
			}

			taskID, err := result.LastInsertId()
			if err != nil {
				t.Fatalf("Failed to retreive inserted task ID: %v", err)
			}

			t.Cleanup(func() {
				_, err := db.Exec(
					"DELETE FROM tasks WHERE id = ?",
					taskID,
				)
				if err != nil {
					t.Errorf("Failed to clean up task: %v", err)
				}
			})

			// UPDATE TASK VIA PATCH CALL
			path := fmt.Sprintf("/tasks/%d", taskID)
			req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(tt.body))
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
			}

			var responseTask Task
			if err := json.NewDecoder(resp.Body).Decode(&responseTask); err != nil {
				t.Fatalf("Failed to decode PATCH response: %v", err)
			}
			if responseTask.Title != tt.wantTitle {
				t.Errorf("New task title %s isnt equal to expected %s", responseTask.Title, tt.wantTitle)
			}
			if responseTask.Completed != tt.wantCompleted {
				t.Errorf(
					"Expected completed=%t, got %t",
					tt.wantCompleted,
					responseTask.Completed,
				)
			}

			if responseTask.ID != int(taskID) {
				t.Errorf(
					"Expected ID=%d, got %d",
					taskID,
					responseTask.ID,
				)
			}

			// SELECT TASK FROM DB
			var task Task
			if err := db.QueryRowContext(ctx, "SELECT title, completed FROM tasks WHERE id = ?", taskID).Scan(&task.Title, &task.Completed); err != nil {
				t.Fatalf("Failed to get new task from DB: %v", err)
			}

			if task.Title != tt.wantTitle {
				t.Errorf("New task title %s isn't equal to expected %s", task.Title, tt.wantTitle)
			}
			if task.Completed != tt.wantCompleted {
				t.Errorf("Expected completed=%t, got %t", tt.wantCompleted, task.Completed)
			}
		})
	}
}

func TestConcurrentPatch(t *testing.T) {
	test := struct {
		name          string
		initialTitle  string
		initialStatus bool
		wantTitle     string
		wantStatus    bool
	}{
		"Test concurrent patch requests",
		"Original",
		false,
		"Updated",
		true,
	}

	var wg sync.WaitGroup
	db := testConnectDB(t)
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /tasks/{id}", updateTaskHandler(db))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	query := "INSERT INTO TASKS (title, completed) VALUES (?, ?)"
	res, err := db.ExecContext(ctx, query, test.initialTitle, test.initialStatus)
	if err != nil {
		t.Fatalf("Failed to insert test data into DB: %v", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		t.Errorf("Failed to get last inserted ID: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		query := "DELETE FROM TASKS WHERE ID = ?"
		if _, err := db.ExecContext(ctx, query, id); err != nil {
			t.Errorf("Failed to cleanup DB: %v", err)
		}
	})

	t.Run(test.name, func(t *testing.T) {
		start := make(chan struct{})
		wg.Go(func() {
			<-start
			path := fmt.Sprintf("/tasks/%d", id)
			req := httptest.NewRequest("PATCH", path, strings.NewReader(fmt.Sprintf(`{"title": "%s"}`, test.wantTitle)))
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
			}

			var responseTask Task
			if err := json.NewDecoder(resp.Body).Decode(&responseTask); err != nil {
				t.Errorf("Failed to decode PATCH response: %v", err)
			}
			if responseTask.Title != test.wantTitle {
				t.Errorf("New task title %s isnt equal to expected %s", responseTask.Title, test.wantTitle)
			}
		})
		wg.Go(func() {
			<-start
			path := fmt.Sprintf("/tasks/%d", id)
			req := httptest.NewRequest("PATCH", path, strings.NewReader(fmt.Sprintf(`{"completed": %t}`, test.wantStatus)))
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body.String())
			}

			var responseTask Task
			if err := json.NewDecoder(resp.Body).Decode(&responseTask); err != nil {
				t.Errorf("Failed to decode PATCH response: %v", err)
			}
			if responseTask.Completed != test.wantStatus {
				t.Errorf(
					"Expected completed=%t, got %t",
					test.wantStatus,
					responseTask.Completed,
				)
			}
		})
		close(start)
		wg.Wait()

		var task Task
		query := "SELECT title, completed FROM tasks WHERE id = ?"
		if err := db.QueryRowContext(ctx, query, id).Scan(&task.Title, &task.Completed); err != nil {
			t.Fatalf("Failed to SELECT task from DB: %v", err)
		}

		if test.wantTitle != task.Title {
			t.Errorf("New task title %s isnt equal to expected %s", task.Title, test.wantTitle)
		}

		if test.wantStatus != task.Completed {
			t.Errorf("New task title %t isnt equal to expected %t", task.Completed, test.wantStatus)
		}
	})
}

func TestTaskValidation(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{
			"Create Empty title",
			http.MethodPost,
			"/tasks",
			`{"title":""}`,
			http.StatusBadRequest,
		},
		{
			"Create Whitespaces title",
			http.MethodPost,
			"/tasks",
			`{"title":"   "}`,
			http.StatusBadRequest,
		},
		{
			"Create 51 characters title",
			http.MethodPost,
			"/tasks",
			fmt.Sprintf(`{"title":"%s"}`, strings.Repeat("a", 51)),
			http.StatusBadRequest,
		},
		{
			"Update empty title",
			http.MethodPatch,
			"/tasks/1",
			`{"title":""}`,
			http.StatusBadRequest,
		},
		{
			"Update whitespace title",
			http.MethodPatch,
			"/tasks/1",
			`{"title":"   "}`,
			http.StatusBadRequest,
		},
		{
			"Update empty JSON",
			http.MethodPatch,
			"/tasks/1",
			"{}",
			http.StatusBadRequest,
		},
		{
			"Create Unknown property",
			http.MethodPost,
			"/tasks",
			`{"title":"Valid","unknown":123}`,
			http.StatusBadRequest,
		},
		{
			"Create extra json",
			http.MethodPost,
			"/tasks",
			`{"title":"Valid"}{"title":"Extra"}`,
			http.StatusBadRequest,
		},
		{
			"Create trailing garbage",
			http.MethodPost,
			"/tasks",
			`{"title":"Valid"}invalid`,
			http.StatusBadRequest,
		},
		{
			"Update multiple JSON",
			http.MethodPatch,
			"/tasks/1",
			`{"title":"Updated"}{"completed":true}`,
			http.StatusBadRequest,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", createTaskHandler(nil))
	mux.HandleFunc("PATCH /tasks/{id}", updateTaskHandler(nil))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			resp := httptest.NewRecorder()
			mux.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("Status code doesnt match: want %d, got %d", tt.wantStatus, resp.Code)
			}
		})
	}
}
