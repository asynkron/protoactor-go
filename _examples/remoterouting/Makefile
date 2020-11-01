start:
	tmux new-session -d -s eg
	tmux split-window -t "eg:0"   -v
	tmux split-window -t "eg:0.0" -h
	tmux send-keys -t "eg:0.0" "go run server/main.go --name node-1 --bind=127.0.0.1:8101" Enter
	tmux send-keys -t "eg:0.1" "go run server/main.go --name node-2 --bind=127.0.0.1:8102" Enter
	tmux send-keys -t "eg:0.2" "go run client/*.go" Enter
	tmux attach -t eg
	tmux kill-session -t eg


stop:
	tmux kill-session -t eg
