
start go run server/main.go --name node-1 --bind=127.0.0.1:8101
start go run server/main.go --name node-2 --bind=127.0.0.1:8102
start go run client/local.go client/main.go