tmux new-session -d -s eg
tmux split-window -t "eg:0"   -v
tmux send-keys -t "eg:0.0" "go run ./... --remoting-port=18080 --clustering-port=28080 --members=localhost:28080,localhost:28081" Enter
tmux send-keys -t "eg:0.1" "go run ./... --remoting-port=18081 --clustering-port=28081 --members=localhost:28080,localhost:28081" Enter
tmux attach -t eg
tmux kill-session -t eg