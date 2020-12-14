loops:=10000
clients:=10
cp:=consul
ttl:=10s

start:
	tmux new-session -d -s eg
	tmux split-window -t "eg:0"   -v
	tmux split-window -t "eg:0.0" -h
	tmux select-pane -t "eg:0.2"
	tmux send-keys -t "eg:0.0" "go run server/main.go --provider $(cp) --ttl $(ttl) --port 8081" Enter
	tmux send-keys -t "eg:0.1" "go run server/main.go --provider $(cp) --ttl $(ttl) --port 8082" Enter
	tmux send-keys -t "eg:0.2" "sleep 3 && go run client/main.go --provider $(cp) --clients $(clients) --loops $(loops)" Enter
	# tmux send-keys -t "eg:0.2" "go run member/main.go" Enter
	tmux attach -t eg
	tmux kill-session -t eg


start-with-etcd:
	make start cp=etcd


stop:
	tmux kill-session -t eg


proto:
	protoc -I=.  --gogoslick_out=. shared/*.proto
	protoc -I=.  --gograinv2_out=.   shared/*.proto
