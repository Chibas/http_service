package task

import (
	"context"
	"database/sql"
	"fmt"
)

func getTaskByID(ctx context.Context, db *sql.DB, id int) (Task, error) {
	var task Task

	err := db.QueryRowContext(ctx, "SELECT id, title, completed FROM tasks WHERE id = ?", id).Scan(&task.ID, &task.Title, &task.Completed)

	if err != nil {
		return Task{}, err
	}
	return task, nil
}

func createTask(ctx context.Context, db *sql.DB, title string) (Task, error) {
	resp, err := db.ExecContext(ctx, "INSERT INTO tasks (title) VALUES (?)", title)
	if err != nil {
		return Task{}, err
	}

	lastInsertedId, err := resp.LastInsertId()
	if err != nil {
		return Task{}, fmt.Errorf("Failed to retreive last inserted id: %w", err)
	}
	return Task{
		ID:        int(lastInsertedId),
		Title:     title,
		Completed: false,
	}, nil
}

func getTasks(ctx context.Context, db *sql.DB) ([]Task, error) {
	var tasks = make([]Task, 0)

	rows, err := db.QueryContext(ctx, "SELECT id, title, completed FROM tasks")
	if err != nil {
		return []Task{}, fmt.Errorf("Couldn't query DB: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var task Task

		if err := rows.Scan(&task.ID, &task.Title, &task.Completed); err != nil {
			return []Task{}, fmt.Errorf("Failed to scan rows: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		return []Task{}, fmt.Errorf("Rows error: %w", err)
	}

	return tasks, nil
}

func deleteTask(ctx context.Context, db *sql.DB, id int) (bool, error) {
	result, err := db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		return false, fmt.Errorf("Failed to delete task: %w", err)
	}

	changedCount, err := result.RowsAffected()

	if err != nil {
		return false, fmt.Errorf("Failed to get rows affected count: %w", err)
	}
	if changedCount == 0 {
		return false, nil
	}

	return true, nil
}

func updateTask(
	ctx context.Context,
	db *sql.DB,
	id int,
	update UpdateTaskRequest,
) (Task, error) {
	_, err := db.ExecContext(ctx, "UPDATE tasks SET title = COALESCE (?, title), completed = COALESCE(?, completed) WHERE id = ?", update.Title, update.Completed, id)
	if err != nil {
		return Task{}, fmt.Errorf("failed to update task: %w", err)
	}
	return getTaskByID(ctx, db, id)
}
