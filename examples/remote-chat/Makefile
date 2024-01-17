start:
	tmux new-session -d -s eg
	tmux split-window -t "eg:0"   -v
	tmux split-window -t "eg:0.1" -h
	tmux send-keys -t "eg:0.0" "go run server/main.go" Enter
	tmux send-keys -t "eg:0.1" "go run client/main.go" Enter
	tmux send-keys -t "eg:0.2" "go run client/main.go" Enter
	tmux attach -t eg
	tmux kill-session -t eg


stop:
	tmux kill-session -t eg
