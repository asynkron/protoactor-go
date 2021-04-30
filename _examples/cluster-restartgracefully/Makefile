cp:=consul
ttl:=10s
loops:=10000
clients:=10
interval:=0ms
env=prod

start:
	tmux new-session -d -s eg
	tmux setenv -t eg PROTO_ACTOR_ENV $(env)
	tmux split-window -t "eg:0"   -v
	tmux split-window -t "eg:0.0" -h -p 66
	tmux split-window -t "eg:0.1" -h -p 50
	tmux select-pane -t "eg:0.3"
	tmux send-keys -t "eg:0.0" "go run server/main.go --provider $(cp) --ttl $(ttl) --port 8081" Enter
	tmux send-keys -t "eg:0.1" "go run server/main.go --provider $(cp) --ttl $(ttl) --port 8082" Enter
	tmux send-keys -t "eg:0.2" "go run server/main.go --provider $(cp) --ttl $(ttl) --port 8083" Enter
	tmux send-keys -t "eg:0.3" "sleep 3 && go run client/main.go --provider $(cp) --clients $(clients) --loops $(loops) --interval $(interval) " Enter
	# tmux send-keys -t "eg:0.2" "go run member/main.go" Enter
	tmux attach -t eg
	tmux kill-session -t eg


start-with-etcd:
	make start cp=etcd


debug:
	PROTO_ACTOR_ENV=dev make start cp=etcd


mock-clients-10w:
	make start cp=etcd clients=100000 loops=10 interval=100ms


mock-clients-20w:
	make start cp=etcd clients=200000 loops=10 interval=100ms


stop:
	tmux kill-session -t eg


proto:
	protoc -I=.  --gogoslick_out=. shared/*.proto
	protoc -I=.  --gograinv2_out=.   shared/*.proto
