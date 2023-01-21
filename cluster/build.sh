protoc -I=../actor --go_out=. --go_opt=paths=source_relative --proto_path=. cluster.proto
protoc -I=../actor --go_out=. --go_opt=paths=source_relative --proto_path=. gossip.proto
protoc -I=../actor --go_out=. --go_opt=paths=source_relative --proto_path=. grain.proto
protoc -I=../actor --go_out=. --go_opt=paths=source_relative --proto_path=. pubsub.proto
protoc -I=../actor --go_out=. --go_opt=paths=source_relative --proto_path=. pubsub_test.proto

