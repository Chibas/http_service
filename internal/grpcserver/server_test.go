package grpcserver

import (
	"context"
	"database/sql"
	"math"
	"net"
	"testing"
	"time"

	taskv1 "github.com/Chibas/http_service/gen/task/v1"
	"github.com/Chibas/http_service/internal/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func testSetup(t *testing.T) (*sql.DB, taskv1.TaskServiceClient) {
	// Connect to the existing MySQL test database
	db := util.TestConnectDB(t)

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	taskServer := NewServer(db)

	taskv1.RegisterTaskServiceServer(grpcServer, taskServer)
	go grpcServer.Serve(listener)
	t.Cleanup(func() {
		grpcServer.Stop()
		listener.Close()
	})
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("create gRPC client connection: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})
	client := taskv1.NewTaskServiceClient(conn)
	return db, client
}

func TestGetTask(t *testing.T) {
	tests := []struct {
		name       string
		wantTitle  string
		wantStatus bool
		wantCode   codes.Code
		requestID  int64
		needsTask  bool
		deleteTask bool
	}{
		{"success", "test GRPC", true, codes.OK, 0, true, false},
		{"not found", "", false, codes.NotFound, 0, true, true},
		{"zero ID", "", false, codes.InvalidArgument, 0, false, false},
		{"negative ID", "", false, codes.InvalidArgument, -1, false, false},
		{"overflow ID", "", false, codes.InvalidArgument, math.MaxInt32 + 1, false, false},
	}

	db, client := testSetup(t)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			id := tt.requestID

			if tt.needsTask {
				res, err := db.ExecContext(
					ctx,
					"INSERT INTO tasks (title, completed) VALUES (?, ?)",
					tt.wantTitle,
					tt.wantStatus,
				)
				if err != nil {
					t.Fatalf("Failed to insert task: %v", err)
				}

				id, err = res.LastInsertId()
				if err != nil {
					t.Fatalf("Failed to get inserted ID: %v", err)
				}

				t.Cleanup(func() {
					if _, err := db.Exec(
						"DELETE FROM tasks WHERE id = ?", id,
					); err != nil {
						t.Errorf("Failed to clean up task: %v", err)
					}
				})

				if tt.deleteTask {
					if _, err := db.ExecContext(
						ctx,
						"DELETE FROM tasks WHERE id = ?", id,
					); err != nil {
						t.Fatalf("Failed to delete task: %v", err)
					}
				}
			}

			resp, err := client.GetTask(
				ctx,
				&taskv1.GetTaskRequest{Id: id},
			)

			gotCode := status.Code(err)
			if gotCode != tt.wantCode {
				t.Fatalf("Expected %s, got %s: %v", tt.wantCode, gotCode, err)
			}

			if tt.wantCode != codes.OK {
				if resp != nil {
					t.Fatalf("Expected nil response, got %+v", resp)
				}
				return
			}

			if resp == nil {
				t.Fatal("Expected task response, got nil")
			}

			if resp.Id != id {
				t.Errorf("ID: got %d, want %d", resp.Id, id)
			}
			if resp.Title != tt.wantTitle {
				t.Errorf("Title: got %q, want %q", resp.Title, tt.wantTitle)
			}
			if resp.Completed != tt.wantStatus {
				t.Errorf("Completed: got %v, want %v", resp.Completed, tt.wantStatus)
			}
		})
	}

}

func TestGetTaskExpiredDeadline(t *testing.T) {
	_, client := testSetup(t)
	ctx, cancel := context.WithDeadline(
		context.Background(),
		time.Now().Add(-time.Second),
	)
	defer cancel()

	resp, err := client.GetTask(
		ctx,
		&taskv1.GetTaskRequest{Id: 1},
	)

	if got := status.Code(err); got != codes.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded, got %s: %v", got, err)
	}

	if resp != nil {
		t.Fatalf("Expected nil response, got %+v", resp)
	}
}

func TestGetTaskCanceled(t *testing.T) {
	_, client := testSetup(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	resp, err := client.GetTask(
		ctx,
		&taskv1.GetTaskRequest{Id: 1},
	)
	if got := status.Code(err); got != codes.Canceled {
		t.Fatalf("Expected canceled, got %s: %v", got, err)
	}

	if resp != nil {
		t.Fatalf("Expected nil response, got %+v", resp)
	}
}

func TestGetTaskDatabaseFailure(t *testing.T) {
	db, client := testSetup(t)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	err := db.Close()
	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	resp, err := client.GetTask(
		ctx,
		&taskv1.GetTaskRequest{Id: 1},
	)

	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("Expected Internal err, got %s: %v", got, err)
	}

	if resp != nil {
		t.Fatalf("Expected nil response, got %+v", resp)
	}
}
