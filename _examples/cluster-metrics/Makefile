start:
	tmux new-session -d -s eg
	tmux split-window -t "eg:0"   -v
	tmux split-window -t "eg:0.0"   -h
	tmux select-pane -t "eg:0.2"
	tmux send-keys -t "eg:0.0" "go run server/main.go -port 8080" Enter
	tmux send-keys -t "eg:0.1" "go run server/main.go -port 8081" Enter
	tmux send-keys -t "eg:0.2" "go run client/main.go" Enter
	tmux attach -t eg
	tmux kill-session -t eg


stop:
	tmux kill-session -t eg


proto:
	protoc -I=.  --gogoslick_out=. shared/*.proto
	protoc --go_out=. --go_opt=paths=source_relative --plugin=$GOPATH/bin/protoc-gen-go-grain --go-grain_out=. --go-grain_opt=paths=source_relative protos.proto

