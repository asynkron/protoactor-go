- Go plugins for the protocol compiler:

1. Install the protocol compiler plugins for Go using the following commands:
    ```
    go install github.com/asynkron/protoactor-go/protobuf/protoc-gen-go-grain@latest
    ```

2. Update your PATH so that the protoc compiler can find the plugins:
    ```
    export PATH="$PATH:$(go env GOPATH)/bin"
    ```
    
3. Compile `.proto` file
   ```
   protoc --go_out=. --go_opt=paths=source_relative \
            --go-grain_out=. --go-grain_opt=paths=source_relative hello.proto
   ```

- If you are using `protoc`, you need to ensure the required dependencies are available to the compiler at compile time. These can be found by manually cloning and copying the relevant files from here and providing them to protoc when running. The files you will need are:
    ```
    protobuf/protoc-gen-go-grain/options/options.proto
    ```
