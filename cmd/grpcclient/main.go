// gRPC test client for the task manager.
//
// Usage examples:
//
//   grpcclient login    -addr :9090 -username admin -password admin12345
//   grpcclient create   -addr :9090 -token <jwt> -title "Demo"
//   grpcclient list     -addr :9090 -token <jwt>
//   grpcclient get      -addr :9090 -token <jwt> -id <task-id>
//   grpcclient update   -addr :9090 -token <jwt> -id <task-id> -status completed
//   grpcclient assign   -addr :9090 -token <jwt> -id <task-id> -user <user-id>
//   grpcclient delete   -addr :9090 -token <jwt> -id <task-id>
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/keploy/taskmanager/proto/taskpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: grpcclient <command> [flags]\n  commands: login, create, list, get, update, delete, assign, unassign")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	addr := fs.String("addr", "localhost:9090", "gRPC server address")
	token := fs.String("token", "", "JWT access token (Bearer)")

	var (
		username = fs.String("username", "", "")
		password = fs.String("password", "", "")
		title    = fs.String("title", "", "")
		desc     = fs.String("desc", "", "")
		priority = fs.String("priority", "", "")
		due      = fs.String("due", "", "RFC3339 due date")
		id       = fs.String("id", "", "task id")
		status   = fs.String("status", "", "")
		ownerID  = fs.String("owner", "", "")
		assignee = fs.String("assignee", "", "")
		user     = fs.String("user", "", "user id (for assign/unassign)")
		limit    = fs.Int("limit", 50, "")
		offset   = fs.Int("offset", 0, "")
	)
	_ = fs.Parse(os.Args[2:])

	conn, err := grpc.NewClient(*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype(taskpb.CodecName)))
	if err != nil {
		die("dial: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if *token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+*token)
	}

	switch cmd {
	case "login":
		out, err := taskpb.NewAuthServiceClient(conn).Login(ctx, &taskpb.LoginRequest{
			Username: *username, Password: *password,
		})
		printOrDie(out, err)
	case "create":
		out, err := taskpb.NewTaskServiceClient(conn).CreateTask(ctx, &taskpb.CreateTaskRequest{
			Title: *title, Description: *desc, Priority: *priority, DueDate: *due,
		})
		printOrDie(out, err)
	case "get":
		out, err := taskpb.NewTaskServiceClient(conn).GetTask(ctx, &taskpb.GetTaskRequest{ID: *id})
		printOrDie(out, err)
	case "list":
		out, err := taskpb.NewTaskServiceClient(conn).ListTasks(ctx, &taskpb.ListTasksRequest{
			OwnerID: *ownerID, AssigneeID: *assignee, Status: *status,
			Limit: int32(*limit), Offset: int32(*offset),
		})
		printOrDie(out, err)
	case "update":
		out, err := taskpb.NewTaskServiceClient(conn).UpdateTask(ctx, &taskpb.UpdateTaskRequest{
			ID: *id, Title: *title, Description: *desc, Status: *status, Priority: *priority, DueDate: *due,
		})
		printOrDie(out, err)
	case "delete":
		out, err := taskpb.NewTaskServiceClient(conn).DeleteTask(ctx, &taskpb.DeleteTaskRequest{ID: *id})
		printOrDie(out, err)
	case "assign":
		out, err := taskpb.NewTaskServiceClient(conn).AssignTask(ctx, &taskpb.AssignTaskRequest{
			TaskID: *id, UserID: *user,
		})
		printOrDie(out, err)
	case "unassign":
		out, err := taskpb.NewTaskServiceClient(conn).UnassignTask(ctx, &taskpb.UnassignTaskRequest{
			TaskID: *id, UserID: *user,
		})
		printOrDie(out, err)
	default:
		die("unknown command: %s", cmd)
	}
}

func printOrDie(v any, err error) {
	if err != nil {
		die("%v", err)
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
