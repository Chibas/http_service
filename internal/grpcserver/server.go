package grpcserver

import (
	"context"
	"database/sql"
	"errors"
	"math"

	taskv1 "github.com/Chibas/http_service/gen/task/v1"
	"github.com/Chibas/http_service/internal/task"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	taskv1.UnimplementedTaskServiceServer
	db *sql.DB
}

func NewServer(db *sql.DB) *Server {
	return &Server{db: db}
}

func (s *Server) GetTask(ctx context.Context, req *taskv1.GetTaskRequest) (*taskv1.TaskResponse, error) {
	id := req.GetId()

	if id <= 0 || id > math.MaxInt32 {
		return nil, status.Error(codes.InvalidArgument, "invalid task ID")
	}

	result, err := task.GetTaskByID(ctx, s.db, int(id))
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, status.Error(codes.NotFound, "task not found")
		case errors.Is(err, context.Canceled):
			return nil, status.Error(codes.Canceled, "request canceled")
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	return &taskv1.TaskResponse{
		Id:        int64(result.ID),
		Title:     result.Title,
		Completed: result.Completed,
	}, nil
}

func (s *Server) GetTasks(ctx context.Context, req *taskv1.GetTasksRequest) (*taskv1.TasksResponse, error) {
	result, err := task.GetTasks(ctx, s.db)
	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			return nil, status.Error(codes.Canceled, "request canceled")
		case errors.Is(err, context.DeadlineExceeded):
			return nil, status.Error(codes.DeadlineExceeded, "deadline exceeded")
		default:
			return nil, status.Error(codes.Internal, "internal server error")
		}
	}

	tasks := make([]*taskv1.TaskResponse, 0, len(result))
	for _, item := range result {
		tasks = append(tasks, &taskv1.TaskResponse{
			Id:        int64(item.ID),
			Title:     item.Title,
			Completed: item.Completed,
		})
	}

	return &taskv1.TasksResponse{
		Tasks: tasks,
	}, nil
}
