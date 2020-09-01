start:
	tmux new-session -d -s eg
	tmux split-window -t "eg:0"   -v
	tmux send-keys -t "eg:0.0" "go run seed/main.go" Enter
	tmux send-keys -t "eg:0.1" "go run member/main.go" Enter
	tmux attach -t eg
	tmux kill-session -t eg


stop:
	tmux kill-session -t eg
