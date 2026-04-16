package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Request struct {
	ID      string      `json:"id"`
	Action  string      `json:"action"`
	Payload interface{} `json:"payload,omitempty"`
}

type Response struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func handle(req Request) Response {
	switch req.Action {
	case "ping":
		return Response{ID: req.ID, Result: "pong"}
	case "readFile":
		// Example: local file access
		path := req.Payload.(map[string]interface{})["path"].(string)
		data, err := os.ReadFile(path)
		if err != nil {
			return Response{ID: req.ID, Error: err.Error()}
		}
		return Response{ID: req.ID, Result: string(data)}
	default:
		return Response{ID: req.ID, Error: "unknown action"}
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue // skip malformed lines
		}
		resp := handle(req)
		out, _ := json.Marshal(resp)
		fmt.Println(string(out)) // stdout
	}
}